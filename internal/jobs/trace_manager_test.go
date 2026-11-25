package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Srinu0725/algotrace/internal/executor"
)

// --------------------------------------------------
// TEST HELPERS
// --------------------------------------------------

func newTestTraceManager(t *testing.T, size int) *Manager {
	t.Helper()

	manager := NewManager(size)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("manager shutdown failed: %v", err)
		}
	})

	return manager
}

func mockRawExecution(
	code string,
) (ExecutionResult, error) {

	return ExecutionResult{
		Stdout:     "30",
		ExitCode:   0,
		DurationMS: 100,
	}, nil
}

func mockTraceExecution(
	code string,
) (ExecutionResult, error) {

	return ExecutionResult{
		ExitCode:   0,
		DurationMS: 150,

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

// --------------------------------------------------
// 1. DEFAULT EXECUTION MODE
// --------------------------------------------------

func TestDefaultExecutionMode(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	err := manager.Submit(Job{
		ID:   "default-mode",
		Code: "print(30)",
	})

	if err != nil {
		t.Fatal(err)
	}

	job, _ := manager.Get("default-mode")

	if job.Mode != ModeRun {
		t.Fatalf("expected run mode, got %q", job.Mode)
	}

	if job.Status != StatusQueued {
		t.Fatalf("unexpected status: %q", job.Status)
	}
}

// --------------------------------------------------
// 2. INVALID EXECUTION MODE
// --------------------------------------------------

func TestInvalidExecutionMode(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	err := manager.Submit(Job{
		ID:   "invalid-mode",
		Mode: "invalid",
	})

	if err == nil {
		t.Fatal("expected invalid mode error")
	}

	if _, exists := manager.Get("invalid-mode"); exists {
		t.Fatal("invalid job should not be stored")
	}
}

// --------------------------------------------------
// 3. DUPLICATE JOB ID
// --------------------------------------------------

func TestDuplicateJobID(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	first := Job{
		ID:   "duplicate",
		Code: "original",
	}

	if err := manager.Submit(first); err != nil {
		t.Fatal(err)
	}

	err := manager.Submit(Job{
		ID:   "duplicate",
		Code: "replacement",
	})

	if err == nil {
		t.Fatal("expected duplicate ID error")
	}

	job, exists := manager.Get("duplicate")

	if !exists {
		t.Fatal("original job was deleted")
	}

	if job.Code != "original" {
		t.Fatal("original job was overwritten")
	}
}

// --------------------------------------------------
// 4. SUCCESSFUL TRACE EXECUTION
// --------------------------------------------------

func TestWorkerSuccessfulTraceExecution(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		mockTraceExecution,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "trace-success",
		Code: "print(30)",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"trace-success",
		StatusCompleted,
	)

	job, _ := manager.Get("trace-success")

	if !job.TraceAvailable {
		t.Fatal("expected trace to be available")
	}

	if job.TraceStatus != "completed" {
		t.Fatalf("unexpected trace status: %q", job.TraceStatus)
	}

	if job.TotalSteps != 1 {
		t.Fatalf("unexpected total steps: %d", job.TotalSteps)
	}

	if job.Stdout != "30" {
		t.Fatalf("unexpected stdout: %q", job.Stdout)
	}

	trace, exists := manager.GetTrace("trace-success")

	if !exists {
		t.Fatal("trace not found")
	}

	if len(trace.Events) != 1 {
		t.Fatal("expected one trace event")
	}

	if string(trace.Events[0].Variables["result"]) != "30" {
		t.Fatal("unexpected variable value")
	}
}

// --------------------------------------------------
// 5. MIXED EXECUTION MODES
// --------------------------------------------------

func TestWorkersSupportMixedModes(t *testing.T) {

	manager := newTestTraceManager(t, 2)

	if err := manager.StartWorkersWithTrace(
		2,
		mockRawExecution,
		mockTraceExecution,
	); err != nil {
		t.Fatal(err)
	}

	for _, job := range []Job{
		{
			ID:   "raw-job",
			Code: "print(30)",
			Mode: ModeRun,
		},
		{
			ID:   "trace-job",
			Code: "print(30)",
			Mode: ModeTrace,
		},
	} {
		if err := manager.Submit(job); err != nil {
			t.Fatal(err)
		}
	}

	waitForStatus(t, manager, "raw-job", StatusCompleted)

	waitForStatus(t, manager, "trace-job", StatusCompleted)

	rawJob, _ := manager.Get("raw-job")

	traceJob, _ := manager.Get("trace-job")

	if rawJob.TraceAvailable {
		t.Fatal("raw execution should not contain a trace")
	}

	if !traceJob.TraceAvailable {
		t.Fatal("traced execution should contain a trace")
	}
}

// --------------------------------------------------
// 6. PYTHON RUNTIME ERROR
// --------------------------------------------------

func TestWorkerFailedTraceExecution(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	traceExecutor := func(
		code string,
	) (ExecutionResult, error) {

		line := 3

		return ExecutionResult{
			ExitCode: 0,

			Trace: &executor.TraceResult{
				Status: "failed",
				Stdout: "Starting",

				Error: &executor.TraceError{
					Type:    "ZeroDivisionError",
					Message: "division by zero",
					Line:    &line,
				},

				TotalSteps: 1,

				Events: []executor.TraceEvent{
					{
						Step:  1,
						Line:  3,
						Event: "exception",
					},
				},
			},
		}, nil
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		traceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "failed-trace",
		Mode: ModeTrace,
		Code: "print(10 / 0)",
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"failed-trace",
		StatusFailed,
	)

	job, _ := manager.Get("failed-trace")

	if !job.TraceAvailable {
		t.Fatal("failed execution should preserve its trace")
	}

	if job.Error != "ZeroDivisionError: division by zero" {
		t.Fatalf("unexpected error: %q", job.Error)
	}

	trace, exists := manager.GetTrace("failed-trace")

	if !exists || trace.Error == nil {
		t.Fatal("expected structured error")
	}

	if trace.Error.Type != "ZeroDivisionError" {
		t.Fatal("unexpected structured error type")
	}
}

// --------------------------------------------------
// 7. TRACE LIMIT
// --------------------------------------------------

func TestWorkerTraceLimitReached(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	traceExecutor := func(
		code string,
	) (ExecutionResult, error) {

		limit := "max_steps"

		return ExecutionResult{
			Trace: &executor.TraceResult{
				Status: "limit_reached",
				Limit:  &limit,
				Events: []executor.TraceEvent{},
			},
		}, nil
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		traceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "trace-limit",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"trace-limit",
		StatusLimitReached,
	)

	job, _ := manager.Get("trace-limit")

	if !job.TraceAvailable {
		t.Fatal("expected partial trace")
	}

	trace, exists := manager.GetTrace("trace-limit")

	if !exists || trace.Limit == nil {
		t.Fatal("expected trace limit information")
	}
}

// --------------------------------------------------
// 8. EXECUTION TIMEOUT
// --------------------------------------------------

func TestWorkerTraceTimeout(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	traceExecutor := func(
		code string,
	) (ExecutionResult, error) {

		return ExecutionResult{
			ExitCode:   -1,
			TimedOut:   true,
			DurationMS: 5000,
		}, nil
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		traceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "trace-timeout",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"trace-timeout",
		StatusTimedOut,
	)

	job, _ := manager.Get("trace-timeout")

	if !job.TimedOut {
		t.Fatal("expected timeout metadata")
	}

	if job.TraceAvailable {
		t.Fatal("unexpected trace after external timeout")
	}
}

// --------------------------------------------------
// 9. INFRASTRUCTURE FAILURE
// --------------------------------------------------

func TestWorkerTraceInfrastructureFailure(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	traceExecutor := func(
		code string,
	) (ExecutionResult, error) {

		return ExecutionResult{},
			errors.New("Docker unavailable")
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		traceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "infra-error",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"infra-error",
		StatusFailed,
	)

	job, _ := manager.Get("infra-error")

	if job.Error != "Docker unavailable" {
		t.Fatalf("unexpected error: %q", job.Error)
	}

	if job.TraceAvailable {
		t.Fatal("unexpected trace")
	}
}

// --------------------------------------------------
// 10. TRACE ISOLATION
// --------------------------------------------------

func TestTraceRetrievalIsolation(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		mockTraceExecution,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "isolation",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"isolation",
		StatusCompleted,
	)

	first, exists := manager.GetTrace("isolation")

	if !exists {
		t.Fatal("trace not found")
	}

	// Mutate the caller's copy.
	first.Status = "modified"

	first.Events[0].Variables["result"] =
		json.RawMessage("999")

	second, exists := manager.GetTrace("isolation")

	if !exists {
		t.Fatal("trace not found")
	}

	if second.Status != "completed" {
		t.Fatal("stored trace was modified")
	}

	if string(second.Events[0].Variables["result"]) != "30" {
		t.Fatal("stored variables were modified")
	}

	raw, exists := manager.GetTraceJSON("isolation")

	if !exists {
		t.Fatal("trace JSON not found")
	}

	raw[0] = 'X'

	fresh, exists := manager.GetTraceJSON("isolation")

	if !exists || !json.Valid(fresh) {
		t.Fatal("stored JSON was modified")
	}
}

// --------------------------------------------------
// 11. CONCURRENT TRACE RETRIEVAL
// --------------------------------------------------

func TestConcurrentTraceRetrieval(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		mockTraceExecution,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "concurrent-trace",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"concurrent-trace",
		StatusCompleted,
	)

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {

		wg.Add(1)

		go func() {

			defer wg.Done()

			for j := 0; j < 100; j++ {

				trace, exists := manager.GetTrace(
					"concurrent-trace",
				)

				if !exists {
					t.Error("trace not found")
					return
				}

				if trace.Status != "completed" {
					t.Error("unexpected trace status")
					return
				}

				// Each goroutine can modify its own copy.
				trace.Events[0].Variables["result"] =
					json.RawMessage("999")
			}
		}()
	}

	wg.Wait()
}

// --------------------------------------------------
// 12. TRACE COUNT LIMIT AND FIFO EVICTION
// --------------------------------------------------

func TestTraceStorageEviction(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	manager.mu.Lock()
	defer manager.mu.Unlock()

	for i := 0; i <= maxStoredTraces; i++ {

		id := fmt.Sprintf("job-%d", i)

		manager.store[id] = Job{
			ID:             id,
			Status:         StatusCompleted,
			TraceAvailable: true,
		}

		data := []byte(`{"status":"completed"}`)

		if !manager.storeTraceLocked(id, data) {
			t.Fatal("could not store trace")
		}
	}

	if len(manager.traces) != maxStoredTraces {
		t.Fatalf(
			"unexpected trace count: %d",
			len(manager.traces),
		)
	}

	if _, exists := manager.traces["job-0"]; exists {
		t.Fatal("oldest trace should have been evicted")
	}

	if manager.store["job-0"].TraceAvailable {
		t.Fatal("evicted trace metadata was not updated")
	}

	if _, exists := manager.store["job-0"]; !exists {
		t.Fatal("job summary should be preserved")
	}
}

// --------------------------------------------------
// 13. TRACE BYTE LIMIT
// --------------------------------------------------

func TestTraceStorageByteLimit(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	manager.mu.Lock()
	defer manager.mu.Unlock()

	// Synthetic payloads test byte accounting,
	// independently of JSON serialization.
	first := make([]byte, maxStoredTraceBytes/2+1)

	second := make([]byte, maxStoredTraceBytes/2+1)

	if !manager.storeTraceLocked("first", first) {
		t.Fatal("could not store first trace")
	}

	if !manager.storeTraceLocked("second", second) {
		t.Fatal("could not store second trace")
	}

	if _, exists := manager.traces["first"]; exists {
		t.Fatal("first trace should have been evicted")
	}

	if manager.traceBytes > maxStoredTraceBytes {
		t.Fatal("trace storage exceeded byte limit")
	}

	if manager.traceBytes != len(second) {
		t.Fatal("incorrect trace byte accounting")
	}
}

// --------------------------------------------------
// 14. MISSING TRACE
// --------------------------------------------------

func TestWorkerMissingTrace(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	traceExecutor := func(
		code string,
	) (ExecutionResult, error) {

		return ExecutionResult{}, nil
	}

	if err := manager.StartWorkersWithTrace(
		1,
		mockRawExecution,
		traceExecutor,
	); err != nil {
		t.Fatal(err)
	}

	if err := manager.Submit(Job{
		ID:   "missing-trace",
		Mode: ModeTrace,
	}); err != nil {
		t.Fatal(err)
	}

	waitForStatus(
		t,
		manager,
		"missing-trace",
		StatusFailed,
	)

	job, _ := manager.Get("missing-trace")

	if job.Error != "Trace execution returned no trace" {
		t.Fatalf("unexpected error: %q", job.Error)
	}
}

// --------------------------------------------------
// 15. TRACE MODE WITH RAW-ONLY WORKERS
// --------------------------------------------------

func TestTraceModeRejectedByRawWorkers(t *testing.T) {

	manager := newTestTraceManager(t, 1)

	if err := manager.StartWorkers(
		1,
		mockRawExecution,
	); err != nil {
		t.Fatal(err)
	}

	err := manager.Submit(Job{
		ID:   "unsupported-trace",
		Mode: ModeTrace,
	})

	if err == nil {
		t.Fatal("expected trace mode rejection")
	}
}
