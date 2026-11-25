//go:build integration

package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Srinu0725/algotrace/internal/executor"
	"github.com/Srinu0725/algotrace/internal/jobs"
)

// --------------------------------------------------
// TEST SETUP
// --------------------------------------------------

func newE2EHandler(t *testing.T) http.Handler {
	t.Helper()

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatal("Docker must be running")
	}

	for _, image := range []string{
		"python:3.12-alpine",
		"algotrace-python:dev",
	} {
		if err := exec.Command(
			"docker", "image", "inspect", image,
		).Run(); err != nil {
			t.Fatalf("Required Docker image missing: %s", image)
		}
	}

	manager := newTraceAPIManager(t)

	if err := manager.StartWorkersWithTrace(
		3,
		runExecutor,
		traceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	return newHandler(manager)
}

// --------------------------------------------------
// SUBMIT ACTUAL PYTHON CODE
// --------------------------------------------------

func submitE2EJob(
	t *testing.T,
	handler http.Handler,
	code string,
	mode string,
) ExecuteResponse {

	t.Helper()

	body, err := json.Marshal(ExecuteRequest{
		Code: code,
		Mode: mode,
	})

	if err != nil {
		t.Fatal(err)
	}

	return submitAPIJob(t, handler, string(body))
}

// --------------------------------------------------
// POLL JOB THROUGH THE HTTP API
// --------------------------------------------------

func waitForE2EJob(
	t *testing.T,
	handler http.Handler,
	id string,
) jobs.Job {

	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {

		rec := apiRequest(
			handler,
			http.MethodGet,
			"/jobs/"+id,
			"",
		)

		if rec.Code != http.StatusOK {
			t.Fatalf(
				"could not retrieve job: %d: %s",
				rec.Code,
				rec.Body.String(),
			)
		}

		var job jobs.Job

		if err := json.Unmarshal(
			rec.Body.Bytes(),
			&job,
		); err != nil {
			t.Fatal(err)
		}

		switch job.Status {

		case jobs.StatusCompleted,
			jobs.StatusFailed,
			jobs.StatusTimedOut,
			jobs.StatusLimitReached:

			return job

		case jobs.StatusQueued,
			jobs.StatusRunning:

			time.Sleep(100 * time.Millisecond)

		default:
			t.Fatalf("unknown job status: %q", job.Status)
		}
	}

	t.Fatal("timed out waiting for job completion")

	return jobs.Job{}
}

// --------------------------------------------------
// RETRIEVE ACTUAL EXECUTION TRACE
// --------------------------------------------------

func getE2ETrace(
	t *testing.T,
	handler http.Handler,
	id string,
) executor.TraceResult {

	t.Helper()

	rec := apiRequest(
		handler,
		http.MethodGet,
		"/jobs/"+id+"/trace",
		"",
	)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"trace retrieval failed: %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	var trace executor.TraceResult

	if err := json.Unmarshal(
		rec.Body.Bytes(),
		&trace,
	); err != nil {
		t.Fatal(err)
	}

	if trace.TotalSteps != len(trace.Events) {
		t.Fatal("trace step count mismatch")
	}

	return trace
}

// --------------------------------------------------
// 1. RAW EXECUTION
// --------------------------------------------------

func TestE2ERawExecution(t *testing.T) {

	handler := newE2EHandler(t)

	response := submitE2EJob(
		t,
		handler,
		"print(10 + 20)",
		jobs.ModeRun,
	)

	job := waitForE2EJob(
		t,
		handler,
		response.JobID,
	)

	if job.Status != jobs.StatusCompleted {
		t.Fatalf("unexpected job: %+v", job)
	}

	if strings.TrimSpace(job.Stdout) != "30" {
		t.Fatalf("unexpected stdout: %q", job.Stdout)
	}

	if job.TraceAvailable {
		t.Fatal("raw execution should not have a trace")
	}
}

// --------------------------------------------------
// 2. BASIC TRACED EXECUTION
// --------------------------------------------------

func TestE2ETracedExecution(t *testing.T) {

	handler := newE2EHandler(t)

	code := `
x = 10
y = 20
result = x + y
print(result)
`

	response := submitE2EJob(
		t,
		handler,
		code,
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusCompleted {
		t.Fatalf("unexpected job: %+v", job)
	}

	if !job.TraceAvailable {
		t.Fatal("expected available trace")
	}

	trace := getE2ETrace(
		t,
		handler,
		response.JobID,
	)

	if trace.Status != "completed" {
		t.Fatalf("unexpected trace status: %s", trace.Status)
	}

	if strings.TrimSpace(trace.Stdout) != "30" {
		t.Fatalf("unexpected output: %q", trace.Stdout)
	}

	found := false

	for _, event := range trace.Events {
		if string(event.Variables["result"]) == "30" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("result variable missing from trace")
	}
}

// --------------------------------------------------
// 3. BUBBLE SORT VISUALIZATION
// --------------------------------------------------

func TestE2EBubbleSort(t *testing.T) {

	handler := newE2EHandler(t)

	code := `
arr = [5, 3, 8, 1, 2]

n = len(arr)

for i in range(n):
    for j in range(n - i - 1):
        if arr[j] > arr[j + 1]:
            arr[j], arr[j + 1] = arr[j + 1], arr[j]

print(arr)
`

	response := submitE2EJob(
		t,
		handler,
		code,
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusCompleted {
		t.Fatalf("unexpected job: %+v", job)
	}

	if strings.TrimSpace(job.Stdout) != "[1, 2, 3, 5, 8]" {
		t.Fatalf("unexpected sorted output: %q", job.Stdout)
	}

	trace := getE2ETrace(
		t,
		handler,
		response.JobID,
	)

	foundSortedArray := false
	mutations := 0

	for _, event := range trace.Events {

		if raw, exists := event.Variables["arr"]; exists {

			var arr []int

			if err := json.Unmarshal(raw, &arr); err == nil {

				if len(arr) == 5 &&
					arr[0] == 1 &&
					arr[1] == 2 &&
					arr[2] == 3 &&
					arr[3] == 5 &&
					arr[4] == 8 {

					foundSortedArray = true
				}
			}
		}

		for _, change := range event.Changes {

			if change.Type == "array_updated" &&
				change.Name == "arr" {

				mutations++
			}
		}
	}

	if !foundSortedArray {
		t.Fatal("sorted array state missing")
	}

	if mutations == 0 {
		t.Fatal("expected array mutation events")
	}

	t.Logf(
		"Bubble Sort: %d trace steps, %d array mutations",
		trace.TotalSteps,
		mutations,
	)
}

// --------------------------------------------------
// 4. RECURSION
// --------------------------------------------------

func TestE2ERecursion(t *testing.T) {

	handler := newE2EHandler(t)

	code := `
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

result = factorial(5)
print(result)
`

	response := submitE2EJob(
		t,
		handler,
		code,
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusCompleted {
		t.Fatalf("unexpected job: %+v", job)
	}

	if strings.TrimSpace(job.Stdout) != "120" {
		t.Fatalf("unexpected factorial result: %q", job.Stdout)
	}

	trace := getE2ETrace(t, handler, response.JobID)

	maxDepth := 0
	calls := 0

	for _, event := range trace.Events {

		if event.Depth > maxDepth {
			maxDepth = event.Depth
		}

		if event.Event == "function_call" &&
			event.Function == "factorial" {
			calls++
		}
	}

	if maxDepth != 5 {
		t.Fatalf("expected depth 5, got %d", maxDepth)
	}

	if calls != 5 {
		t.Fatalf("expected 5 calls, got %d", calls)
	}
}

// --------------------------------------------------
// 5. RUNTIME ERROR
// --------------------------------------------------

func TestE2ERuntimeError(t *testing.T) {

	handler := newE2EHandler(t)

	response := submitE2EJob(
		t,
		handler,
		"print('Starting')\nx = 10 / 0",
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusFailed {
		t.Fatalf("unexpected job: %+v", job)
	}

	if !job.TraceAvailable {
		t.Fatal("expected failed execution trace")
	}

	trace := getE2ETrace(t, handler, response.JobID)

	if trace.Status != "failed" {
		t.Fatal("expected failed trace status")
	}

	if trace.Error == nil ||
		trace.Error.Type != "ZeroDivisionError" {

		t.Fatal("expected ZeroDivisionError")
	}

	if strings.TrimSpace(trace.Stdout) != "Starting" {
		t.Fatal("output before failure was not preserved")
	}
}

// --------------------------------------------------
// 6. SYNTAX ERROR
// --------------------------------------------------

func TestE2ESyntaxError(t *testing.T) {

	handler := newE2EHandler(t)

	response := submitE2EJob(
		t,
		handler,
		"def invalid_function(",
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusFailed {
		t.Fatalf("unexpected job: %+v", job)
	}

	trace := getE2ETrace(t, handler, response.JobID)

	if trace.Error == nil ||
		trace.Error.Type != "SyntaxError" {

		t.Fatal("expected SyntaxError")
	}

	if trace.TotalSteps != 0 {
		t.Fatal("syntax error should not produce execution steps")
	}
}

// --------------------------------------------------
// 7. TRACE LIMIT
// --------------------------------------------------

func TestE2ETraceLimit(t *testing.T) {

	handler := newE2EHandler(t)

	response := submitE2EJob(
		t,
		handler,
		"while True:\n    pass",
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusLimitReached {
		t.Fatalf("unexpected job: %+v", job)
	}

	trace := getE2ETrace(t, handler, response.JobID)

	if trace.Limit == nil {
		t.Fatal("expected trace limit information")
	}

	if trace.TotalSteps > 10000 {
		t.Fatal("trace exceeded maximum step count")
	}
}

// --------------------------------------------------
// 8. EXTERNAL EXECUTION TIMEOUT
// --------------------------------------------------

func TestE2ETimeout(t *testing.T) {

	handler := newE2EHandler(t)

	response := submitE2EJob(
		t,
		handler,
		"import time\ntime.sleep(30)",
		jobs.ModeTrace,
	)

	job := waitForE2EJob(t, handler, response.JobID)

	if job.Status != jobs.StatusTimedOut {
		t.Fatalf("unexpected job: %+v", job)
	}

	if !job.TimedOut {
		t.Fatal("expected timeout metadata")
	}

	if job.TraceAvailable {
		t.Fatal("unexpected complete trace after timeout")
	}

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
// 9. MULTIPLE INDEPENDENT JOBS
// --------------------------------------------------

func TestE2EMultipleJobs(t *testing.T) {

	handler := newE2EHandler(t)

	first := submitE2EJob(
		t,
		handler,
		"print(100)",
		jobs.ModeRun,
	)

	second := submitE2EJob(
		t,
		handler,
		"print(200)",
		jobs.ModeTrace,
	)

	third := submitE2EJob(
		t,
		handler,
		"print(300)",
		jobs.ModeTrace,
	)

	expected := map[string]string{
		first.JobID:  "100",
		second.JobID: "200",
		third.JobID:  "300",
	}

	if len(expected) != 3 {
		t.Fatal("job IDs are not unique")
	}

	for id, output := range expected {

		job := waitForE2EJob(t, handler, id)

		if job.Status != jobs.StatusCompleted {
			t.Fatalf("unexpected job: %+v", job)
		}

		if strings.TrimSpace(job.Stdout) != output {
			t.Fatalf(
				"job %s: expected %q, got %q",
				id,
				output,
				job.Stdout,
			)
		}
	}

	for _, response := range []ExecuteResponse{
		second,
		third,
	} {
		trace := getE2ETrace(t, handler, response.JobID)

		if strings.TrimSpace(trace.Stdout) !=
			expected[response.JobID] {

			t.Fatal("trace belongs to a different job")
		}
	}
}
