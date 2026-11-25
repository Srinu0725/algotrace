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

const maxRequestBody = 64 * 1024 // 64 KB

type ExecuteRequest struct {
	Code string `json:"code"`
}

type ExecuteResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

func main() {
	manager := jobs.NewManager(100)
	workerCount := 3

	err := manager.StartWorkers(workerCount, func(code string) (jobs.ExecutionResult, error) {
		result, err := executor.RunPython(code)

		return jobs.ExecutionResult{
			Stdout:          result.Stdout,
			Stderr:          result.Stderr,
			ExitCode:        result.ExitCode,
			DurationMS:      result.Duration.Milliseconds(),
			StdoutTruncated: result.StdoutTruncated,
			StderrTruncated: result.StderrTruncated,
		}, err
	})

	if err != nil {
		log.Fatalf("Could not start job workers: %v", err)
	}

	server := &http.Server{
		Addr:              ":8080",
		Handler:           newHandler(manager),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Listen for Ctrl+C and operating-system termination signals.
	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)

	go func() {
		log.Println("AlgoTrace server running on :8080")
		serverErrors <- server.ListenAndServe()
	}()

	// Wait until either the server stops or a termination signal arrives.
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped unexpectedly: %v", err)
		}

	case <-signalCtx.Done():
		log.Println("Shutdown signal received")
	}

	// Allow up to 15 seconds for graceful shutdown.
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	// First stop accepting new HTTP requests and finish in-flight requests.
	serverCtx, serverCancel := context.WithTimeout(shutdownCtx, 5*time.Second)
	defer serverCancel()

	if err := server.Shutdown(serverCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
		server.Close()
	} else {
		log.Println("HTTP server stopped gracefully")
	}

	// Then allow queued and running jobs to finish.
	if err := manager.Shutdown(shutdownCtx); err != nil {
		log.Printf("Job manager shutdown error: %v", err)
	} else {
		log.Println("All job workers stopped gracefully")
	}

	log.Println("AlgoTrace shutdown complete")
}

// newHandler creates and configures all HTTP routes.
func newHandler(manager *jobs.Manager) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "AlgoTrace is running")
	})

	mux.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit the entire JSON request body to 64 KB.
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var req ExecuteRequest

		if err := decoder.Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError

			if errors.As(err, &maxBytesErr) {
				http.Error(
					w,
					"Request body exceeds the 64 KB limit",
					http.StatusRequestEntityTooLarge,
				)
				return
			}

			http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
			return
		}

		// Reject additional JSON values or trailing invalid content.
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(
				w,
				"Request body must contain exactly one JSON object",
				http.StatusBadRequest,
			)
			return
		}

		if strings.TrimSpace(req.Code) == "" {
			http.Error(w, "Code cannot be empty", http.StatusBadRequest)
			return
		}

		now := time.Now()

		j := jobs.Job{
			ID:        uuid.NewString(),
			Code:      req.Code,
			Status:    "queued",
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := manager.Submit(j); err != nil {
			http.Error(
				w,
				"Job queue is full; try again later",
				http.StatusServiceUnavailable,
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)

		json.NewEncoder(w).Encode(ExecuteResponse{
			JobID:  j.ID,
			Status: j.Status,
		})
	})

	mux.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/jobs/")

		if id == "" || strings.Contains(id, "/") {
			http.Error(w, "Invalid job ID", http.StatusBadRequest)
			return
		}

		j, ok := manager.Get(id)

		if !ok {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(j)
	})

	return mux
}
