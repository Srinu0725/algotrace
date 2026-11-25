package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Srinu0725/algotrace/internal/executor"
	"github.com/Srinu0725/algotrace/internal/jobs"
)

// --------------------------------------------------
// TEST HELPERS
// --------------------------------------------------

func newTraceAPIManager(t *testing.T) *jobs.Manager {

	t.Helper()

	manager := jobs.NewManager(10)

	t.Cleanup(func() {

		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		defer cancel()

		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("shutdown failed: %v", err)
		}
	})

	return manager
}

func mockRunExecutor(
	code string,
) (jobs.ExecutionResult, error) {

	return jobs.ExecutionResult{
		Stdout:   "30",
		ExitCode: 0,
	}, nil
}

func mockTraceExecutor(
	code string,
) (jobs.ExecutionResult, error) {

	return jobs.ExecutionResult{
		ExitCode:   0,
		DurationMS: 100,

		Trace: &executor.TraceResult{
			Status:         "completed",
			Stdout:         "30",
			TotalSteps:     1,
			TraceSizeBytes: 100,

			Events: []executor.TraceEvent{
				{
					Step:     1,
					Line:     3,
					Event:    "line",
					Function: "<module>",

					Variables: map[string]json.RawMessage{
						"result": json.RawMessage("30"),
					},
				},
			},
		},
	}, nil
}

func apiRequest(
	handler http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {

	req := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	return rec
}

func submitAPIJob(
	t *testing.T,
	handler http.Handler,
	body string,
) ExecuteResponse {

	t.Helper()

	rec := apiRequest(
		handler,
		http.MethodPost,
		"/execute",
		body,
	)

	if rec.Code != http.StatusAccepted {

		t.Fatalf(
			"expected 202, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	var response ExecuteResponse

	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&response,
	); err != nil {

		t.Fatal(err)
	}

	if response.JobID == "" {
		t.Fatal("expected job ID")
	}

	if rec.Header().Get("Location") !=
		"/jobs/"+response.JobID {

		t.Fatal("incorrect Location header")
	}

	return response
}

func waitForAPIJob(
	t *testing.T,
	manager *jobs.Manager,
	id string,
	status string,
) jobs.Job {

	t.Helper()

	deadline := time.Now().Add(
		3 * time.Second,
	)

	for time.Now().Before(deadline) {

		job, exists := manager.Get(id)

		if exists && job.Status == status {
			return job
		}

		time.Sleep(10 * time.Millisecond)
	}

	job, _ := manager.Get(id)

	t.Fatalf(
		"expected status %q, got %q",
		status,
		job.Status,
	)

	return jobs.Job{}
}

// --------------------------------------------------
// 1. DEFAULT MODE
// --------------------------------------------------

func TestExecuteDefaultMode(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)"}`,
	)

	if response.Mode != jobs.ModeRun {
		t.Fatalf("unexpected mode: %q", response.Mode)
	}

	job, exists := manager.Get(response.JobID)

	if !exists || job.Mode != jobs.ModeRun {
		t.Fatal("expected raw execution job")
	}
}

// --------------------------------------------------
// 2. TRACE MODE SUBMISSION
// --------------------------------------------------

func TestExecuteAcceptsTraceMode(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)","mode":"trace"}`,
	)

	if response.Mode != jobs.ModeTrace {
		t.Fatal("expected trace mode")
	}

	job, exists := manager.Get(response.JobID)

	if !exists || job.Mode != jobs.ModeTrace {
		t.Fatal("trace job was not stored")
	}
}

// --------------------------------------------------
// 3. INVALID MODE
// --------------------------------------------------

func TestExecuteRejectsInvalidMode(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	rec := apiRequest(
		handler,
		http.MethodPost,
		"/execute",
		`{"code":"print(30)","mode":"invalid"}`,
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --------------------------------------------------
// 4. JOB SUMMARY
// --------------------------------------------------

func TestGetTraceJobSummary(t *testing.T) {

	manager := newTraceAPIManager(t)

	if err := manager.StartWorkersWithTrace(
		1,
		mockRunExecutor,
		mockTraceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)","mode":"trace"}`,
	)

	waitForAPIJob(
		t,
		manager,
		response.JobID,
		jobs.StatusCompleted,
	)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+response.JobID,
		"",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var job jobs.Job

	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&job,
	); err != nil {
		t.Fatal(err)
	}

	if !job.TraceAvailable {
		t.Fatal("expected available trace")
	}

	if job.TotalSteps != 1 {
		t.Fatal("unexpected step count")
	}

	if job.Stdout != "30" {
		t.Fatal("unexpected stdout")
	}

	// Job summaries must not contain trace events.
	var fields map[string]json.RawMessage

	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&fields,
	); err != nil {
		t.Fatal(err)
	}

	if _, exists := fields["events"]; exists {
		t.Fatal("job summary contains detailed trace events")
	}
}

// --------------------------------------------------
// 5. SUCCESSFUL TRACE RETRIEVAL
// --------------------------------------------------

func TestGetExecutionTrace(t *testing.T) {

	manager := newTraceAPIManager(t)

	if err := manager.StartWorkersWithTrace(
		1,
		mockRunExecutor,
		mockTraceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)","mode":"trace"}`,
	)

	waitForAPIJob(
		t,
		manager,
		response.JobID,
		jobs.StatusCompleted,
	)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+response.JobID+"/trace",
		"",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var trace executor.TraceResult

	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&trace,
	); err != nil {
		t.Fatal(err)
	}

	if trace.Status != "completed" {
		t.Fatal("unexpected trace status")
	}

	if len(trace.Events) != 1 {
		t.Fatal("expected one trace event")
	}

	if string(trace.Events[0].Variables["result"]) != "30" {
		t.Fatal("unexpected variable value")
	}

	if rec.Header().Get("Content-Type") !=
		"application/json" {

		t.Fatal("expected JSON content type")
	}
}

// --------------------------------------------------
// 6. TRACE NOT READY
// --------------------------------------------------

func TestGetTraceBeforeExecution(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)","mode":"trace"}`,
	)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+response.JobID+"/trace",
		"",
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

// --------------------------------------------------
// 7. UNKNOWN JOB
// --------------------------------------------------

func TestGetTraceForUnknownJob(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/unknown-job/trace",
		"",
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --------------------------------------------------
// 8. RAW JOB TRACE REQUEST
// --------------------------------------------------

func TestGetTraceForRawJob(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)","mode":"run"}`,
	)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+response.JobID+"/trace",
		"",
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --------------------------------------------------
// 9. FAILED PYTHON EXECUTION
// --------------------------------------------------

func TestGetFailedExecutionTrace(t *testing.T) {

	manager := newTraceAPIManager(t)

	failedExecutor := func(
		code string,
	) (jobs.ExecutionResult, error) {

		line := 1

		return jobs.ExecutionResult{
			Trace: &executor.TraceResult{
				Status: "failed",

				Error: &executor.TraceError{
					Type:    "ZeroDivisionError",
					Message: "division by zero",
					Line:    &line,
				},

				Events: []executor.TraceEvent{},
			},
		}, nil
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRunExecutor,
		failedExecutor,
	); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"print(10/0)","mode":"trace"}`,
	)

	waitForAPIJob(
		t,
		manager,
		response.JobID,
		jobs.StatusFailed,
	)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+response.JobID+"/trace",
		"",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var trace executor.TraceResult

	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&trace,
	); err != nil {
		t.Fatal(err)
	}

	if trace.Status != "failed" {
		t.Fatal("expected failed trace")
	}

	if trace.Error == nil ||
		trace.Error.Type != "ZeroDivisionError" {

		t.Fatal("expected structured Python error")
	}
}

// --------------------------------------------------
// 10. TIMED-OUT EXECUTION
// --------------------------------------------------

func TestGetTraceAfterTimeout(t *testing.T) {

	manager := newTraceAPIManager(t)

	timeoutExecutor := func(
		code string,
	) (jobs.ExecutionResult, error) {

		return jobs.ExecutionResult{
			TimedOut:   true,
			ExitCode:   -1,
			DurationMS: 5000,
		}, nil
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRunExecutor,
		timeoutExecutor,
	); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(manager)

	response := submitAPIJob(
		t,
		handler,
		`{"code":"while True: pass","mode":"trace"}`,
	)

	waitForAPIJob(
		t,
		manager,
		response.JobID,
		jobs.StatusTimedOut,
	)

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+response.JobID+"/trace",
		"",
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --------------------------------------------------
// 11. UNSUPPORTED HTTP METHOD
// --------------------------------------------------

func TestTraceEndpointRejectsPOST(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	rec := apiRequest(
		handler,
		http.MethodPost,
		"/jobs/job-1/trace",
		"",
	)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	if rec.Header().Get("Allow") != http.MethodGet {
		t.Fatal("expected Allow header")
	}
}

// --------------------------------------------------
// 12. INVALID JOB PATHS
// --------------------------------------------------

func TestInvalidTracePaths(t *testing.T) {

	manager := newTraceAPIManager(t)

	handler := newHandler(manager)

	paths := []string{
		"/jobs/job-1/invalid",
		"/jobs/job-1/trace/extra",
		"/jobs/job-1/",
	}

	for _, path := range paths {

		t.Run(path, func(t *testing.T) {

			rec := apiRequest(
				handler,
				http.MethodGet,
				path,
				"",
			)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
		})
	}
}

// --------------------------------------------------
// 13. QUEUE CAPACITY
// --------------------------------------------------

func TestExecuteReturns503WhenQueueIsFull(t *testing.T) {

	manager := jobs.NewManager(1)

	handler := newHandler(manager)

	submitAPIJob(
		t,
		handler,
		`{"code":"print(1)"}`,
	)

	rec := apiRequest(
		handler,
		http.MethodPost,
		"/execute",
		`{"code":"print(2)"}`,
	)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

// --------------------------------------------------
// 14. TRACE NO LONGER AVAILABLE
// --------------------------------------------------

func TestGetEvictedTrace(t *testing.T) {

	manager := newTraceAPIManager(t)

	if err := manager.StartWorkersWithTrace(
		1,
		mockRunExecutor,
		mockTraceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	handler := newHandler(manager)

	first := submitAPIJob(
		t,
		handler,
		`{"code":"print(30)","mode":"trace"}`,
	)

	waitForAPIJob(
		t,
		manager,
		first.JobID,
		jobs.StatusCompleted,
	)

	// Current retention policy stores at most 100 traces.
	for i := 0; i < 100; i++ {

		response := submitAPIJob(
			t,
			handler,
			`{"code":"print(30)","mode":"trace"}`,
		)

		waitForAPIJob(
			t,
			manager,
			response.JobID,
			jobs.StatusCompleted,
		)
	}

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+first.JobID+"/trace",
		"",
	)

	if rec.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", rec.Code)
	}
}
