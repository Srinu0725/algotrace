package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Srinu0725/algotrace/internal/executor"
	"github.com/Srinu0725/algotrace/internal/jobs"
	"github.com/google/uuid"
)

const maxRequestBody = 64 * 1024

// --------------------------------------------------
// API MODELS
// --------------------------------------------------

type ExecuteRequest struct {
	Code string `json:"code"`
	Mode string `json:"mode,omitempty"`
}

type ExecuteResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Mode   string `json:"mode"`
}

// --------------------------------------------------
// EXECUTOR ADAPTERS
// --------------------------------------------------

// Converts raw Docker execution results into job results.
func runExecutor(code string) (jobs.ExecutionResult, error) {

	result, err := executor.RunPython(code)

	return jobs.ExecutionResult{
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		ExitCode:        result.ExitCode,
		DurationMS:      result.Duration.Milliseconds(),
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
		TimedOut:        err == nil && result.ExitCode == -1,
	}, err
}

// Converts structured Python traces into job results.
func traceExecutor(code string) (jobs.ExecutionResult, error) {

	result, err := executor.RunPythonTrace(code)

	execution := jobs.ExecutionResult{
		ExitCode:        result.ExitCode,
		DurationMS:      result.Duration.Milliseconds(),
		TimedOut:        result.TimedOut,
		Stderr:          result.DockerStderr,
		StderrTruncated: result.StderrTruncated,
		Trace:           result.Trace,
	}

	if result.Trace != nil {
		execution.Stdout = result.Trace.Stdout
		execution.Stderr = result.Trace.Stderr

		execution.StdoutTruncated =
			result.Trace.StdoutTruncated

		execution.StderrTruncated =
			result.Trace.StderrTruncated
	}

	return execution, err
}

// --------------------------------------------------
// SERVER STARTUP
// --------------------------------------------------

func main() {

	manager := jobs.NewManager(100)

	workerCount := 3

	err := manager.StartWorkersWithTrace(
		workerCount,
		runExecutor,
		traceExecutor,
	)

	if err != nil {
		log.Fatalf(
			"Could not start job workers: %v",
			err,
		)
	}

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           newHandler(manager),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)

	go func() {

		log.Println(
			"AlgoTrace server running on 127.0.0.1:8080",
		)

		log.Printf(
			"Started %d execution workers",
			workerCount,
		)

		log.Println(
			"Supported execution modes: run, trace",
		)

		serverErrors <- server.ListenAndServe()
	}()

	select {

	case err := <-serverErrors:

		if err != nil &&
			!errors.Is(err, http.ErrServerClosed) {

			log.Printf(
				"HTTP server stopped unexpectedly: %v",
				err,
			)
		}

	case <-signalCtx.Done():

		log.Println("Shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	serverCtx, serverCancel := context.WithTimeout(
		shutdownCtx,
		5*time.Second,
	)
	defer serverCancel()

	if err := server.Shutdown(serverCtx); err != nil {

		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)

		server.Close()

	} else {

		log.Println(
			"HTTP server stopped gracefully",
		)
	}

	if err := manager.Shutdown(shutdownCtx); err != nil {

		log.Printf(
			"Job manager shutdown error: %v",
			err,
		)

	} else {

		log.Println(
			"All job workers stopped gracefully",
		)
	}

	log.Println("AlgoTrace shutdown complete")
}

// --------------------------------------------------
// RESPONSE HELPERS
// --------------------------------------------------

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.Header().Set(
		"Cache-Control",
		"no-store",
	)

	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON response error: %v", err)
	}
}

func methodNotAllowed(
	w http.ResponseWriter,
	method string,
) {

	w.Header().Set("Allow", method)

	http.Error(
		w,
		"Method not allowed",
		http.StatusMethodNotAllowed,
	)
}

func writeRequestError(
	w http.ResponseWriter,
	err error,
	message string,
) {

	var maxBytesErr *http.MaxBytesError

	if errors.As(err, &maxBytesErr) {

		http.Error(
			w,
			"Request body exceeds the 64 KB limit",
			http.StatusRequestEntityTooLarge,
		)

		return
	}

	http.Error(
		w,
		message,
		http.StatusBadRequest,
	)
}

// --------------------------------------------------
// HEALTH HANDLER
// --------------------------------------------------

func healthHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodGet {

		methodNotAllowed(w, http.MethodGet)

		return
	}

	w.Header().Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	w.WriteHeader(http.StatusOK)

	fmt.Fprintln(w, "AlgoTrace is running")
}

// --------------------------------------------------
// EXECUTION HANDLER
// --------------------------------------------------

func executeHandler(
	manager *jobs.Manager,
) http.HandlerFunc {

	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {

		if r.Method != http.MethodPost {

			methodNotAllowed(w, http.MethodPost)

			return
		}

		r.Body = http.MaxBytesReader(
			w,
			r.Body,
			maxRequestBody,
		)

		defer r.Body.Close()

		decoder := json.NewDecoder(r.Body)

		decoder.DisallowUnknownFields()

		var req ExecuteRequest

		if err := decoder.Decode(&req); err != nil {

			writeRequestError(
				w,
				err,
				"Invalid JSON request body",
			)

			return
		}

		// Reject multiple JSON values and trailing content.
		var extra any

		if err := decoder.Decode(&extra); err != io.EOF {

			writeRequestError(
				w,
				err,
				"Request body must contain exactly one JSON object",
			)

			return
		}

		if strings.TrimSpace(req.Code) == "" {

			http.Error(
				w,
				"Code cannot be empty",
				http.StatusBadRequest,
			)

			return
		}

		// Default execution mode preserves compatibility.
		if req.Mode == "" {
			req.Mode = jobs.ModeRun
		}

		switch req.Mode {

		case jobs.ModeRun, jobs.ModeTrace:
			// Valid execution modes.

		default:

			http.Error(
				w,
				"Invalid execution mode; expected run or trace",
				http.StatusBadRequest,
			)

			return
		}

		now := time.Now()

		job := jobs.Job{
			ID:        uuid.NewString(),
			Code:      req.Code,
			Mode:      req.Mode,
			Status:    jobs.StatusQueued,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := manager.Submit(job); err != nil {

			log.Printf("Job submission failed: %v", err)

			http.Error(
				w,
				"Job submission unavailable; try again later",
				http.StatusServiceUnavailable,
			)

			return
		}

		w.Header().Set(
			"Location",
			"/jobs/"+job.ID,
		)

		writeJSON(
			w,
			http.StatusAccepted,
			ExecuteResponse{
				JobID:  job.ID,
				Status: job.Status,
				Mode:   job.Mode,
			},
		)
	}
}

// --------------------------------------------------
// TRACE HANDLER
// --------------------------------------------------

func serveJobTrace(
	w http.ResponseWriter,
	manager *jobs.Manager,
	job jobs.Job,
) {

	if job.Mode != jobs.ModeTrace {

		http.Error(
			w,
			"This job was not executed in trace mode",
			http.StatusNotFound,
		)

		return
	}

	switch job.Status {

	case jobs.StatusQueued, jobs.StatusRunning:

		http.Error(
			w,
			"Execution trace is not ready",
			http.StatusConflict,
		)

		return
	}

	data, exists := manager.GetTraceJSON(job.ID)

	if !exists {

		// A previously generated trace may have been
		// removed by the retention policy.
		if job.TraceStatus != "" {

			http.Error(
				w,
				"Execution trace is no longer available",
				http.StatusGone,
			)

			return
		}

		http.Error(
			w,
			"No execution trace is available",
			http.StatusNotFound,
		)

		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.Header().Set(
		"Cache-Control",
		"no-store",
	)

	w.WriteHeader(http.StatusOK)

	// The trace is already serialized.
	// Avoid unnecessary decoding and encoding.
	if _, err := w.Write(data); err != nil {
		log.Printf("Trace response error: %v", err)
	}
}

// --------------------------------------------------
// JOB HANDLER
// --------------------------------------------------

func jobHandler(
	manager *jobs.Manager,
) http.HandlerFunc {

	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {

		if r.Method != http.MethodGet {

			methodNotAllowed(w, http.MethodGet)

			return
		}

		path := strings.TrimPrefix(
			r.URL.Path,
			"/jobs/",
		)

		parts := strings.Split(path, "/")

		if len(parts) == 0 ||
			parts[0] == "" ||
			len(parts) > 2 {

			http.Error(
				w,
				"Invalid job path",
				http.StatusBadRequest,
			)

			return
		}

		id := parts[0]

		isTraceRequest := len(parts) == 2

		if isTraceRequest && parts[1] != "trace" {

			http.Error(
				w,
				"Invalid job endpoint",
				http.StatusBadRequest,
			)

			return
		}

		job, exists := manager.Get(id)

		if !exists {

			http.Error(
				w,
				"Job not found",
				http.StatusNotFound,
			)

			return
		}

		if isTraceRequest {

			serveJobTrace(
				w,
				manager,
				job,
			)

			return
		}

		writeJSON(
			w,
			http.StatusOK,
			job,
		)
	}
}

// --------------------------------------------------
// ROUTE REGISTRATION
// --------------------------------------------------

func newHandler(
	manager *jobs.Manager,
) http.Handler {

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/health",
		healthHandler,
	)

	mux.HandleFunc(
		"/execute",
		executeHandler(manager),
	)

	mux.HandleFunc(
		"/jobs/",
		jobHandler(manager),
	)

	return mux
}
