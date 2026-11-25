package jobs

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubmitAndGetJob(t *testing.T) {
	manager := NewManager(10)

	job := Job{
		ID:     "job-1",
		Code:   "print(10)",
		Status: "queued",
	}

	err := manager.Submit(job)
	if err != nil {
		t.Fatalf("unexpected submit error: %v", err)
	}

	got, ok := manager.Get("job-1")

	if !ok {
		t.Fatal("expected job to exist")
	}

	if got.Status != "queued" {
		t.Fatalf("expected queued status, got %q", got.Status)
	}
}

func TestSubmitQueueFull(t *testing.T) {
	manager := NewManager(1)

	first := Job{ID: "job-1", Status: "queued"}
	second := Job{ID: "job-2", Status: "queued"}

	if err := manager.Submit(first); err != nil {
		t.Fatalf("unexpected first submit error: %v", err)
	}

	err := manager.Submit(second)

	if err == nil {
		t.Fatal("expected queue-full error")
	}

	_, exists := manager.Get("job-2")

	if exists {
		t.Fatal("job should not remain stored after failed submission")
	}
}

func TestWorkerSuccessfulExecution(t *testing.T) {
	manager := NewManager(1)

	job := Job{
		ID:     "job-1",
		Code:   "print(10 + 20)",
		Status: "queued",
	}

	if err := manager.Submit(job); err != nil {
		t.Fatal(err)
	}

	manager.StartWorker(func(code string) (ExecutionResult, error) {
		return ExecutionResult{
			Stdout:     "30",
			ExitCode:   0,
			DurationMS: 100,
		}, nil
	})

	waitForStatus(t, manager, "job-1", "completed")

	got, _ := manager.Get("job-1")

	if got.Stdout != "30" {
		t.Fatalf("expected stdout 30, got %q", got.Stdout)
	}

	if got.DurationMS != 100 {
		t.Fatalf("expected duration 100, got %d", got.DurationMS)
	}
}

func TestWorkerFailedExecution(t *testing.T) {
	manager := NewManager(1)

	job := Job{
		ID:     "job-2",
		Code:   "print(10 / 0)",
		Status: "queued",
	}

	if err := manager.Submit(job); err != nil {
		t.Fatal(err)
	}

	manager.StartWorker(func(code string) (ExecutionResult, error) {
		return ExecutionResult{
			Stderr:   "ZeroDivisionError",
			ExitCode: 1,
		}, nil
	})

	waitForStatus(t, manager, "job-2", "failed")

	got, _ := manager.Get("job-2")

	if got.Stderr != "ZeroDivisionError" {
		t.Fatalf("unexpected stderr: %q", got.Stderr)
	}
}

func TestWorkerExecutionError(t *testing.T) {
	manager := NewManager(1)

	job := Job{
		ID:     "job-3",
		Code:   "print(10)",
		Status: "queued",
	}

	if err := manager.Submit(job); err != nil {
		t.Fatal(err)
	}

	manager.StartWorker(func(code string) (ExecutionResult, error) {
		return ExecutionResult{}, errors.New("Docker unavailable")
	})

	waitForStatus(t, manager, "job-3", "failed")

	got, _ := manager.Get("job-3")

	if got.Error != "Docker unavailable" {
		t.Fatalf("unexpected error: %q", got.Error)
	}
}

func TestWorkerPropagatesTruncationMetadata(t *testing.T) {
	manager := NewManager(1)

	job := Job{
		ID:     "job-4",
		Code:   "print('large output')",
		Status: "queued",
	}

	if err := manager.Submit(job); err != nil {
		t.Fatal(err)
	}

	manager.StartWorker(func(code string) (ExecutionResult, error) {
		return ExecutionResult{
			Stdout:          "output",
			ExitCode:        0,
			StdoutTruncated: true,
			StderrTruncated: true,
		}, nil
	})

	waitForStatus(t, manager, "job-4", "completed")

	got, _ := manager.Get("job-4")

	if !got.StdoutTruncated {
		t.Fatal("expected stdout truncation metadata")
	}

	if !got.StderrTruncated {
		t.Fatal("expected stderr truncation metadata")
	}
}

func waitForStatus(
	t *testing.T,
	manager *Manager,
	id string,
	expected string,
) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		job, ok := manager.Get(id)

		if !ok {
			t.Fatalf("job %s not found", id)
		}

		if job.Status == expected {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	job, _ := manager.Get(id)

	t.Fatalf(
		"timed out waiting for status %q; current status: %q",
		expected,
		job.Status,
	)
}

func TestWorkersExecuteJobsConcurrently(t *testing.T) {
	manager := NewManager(3)

	var activeWorkers int32
	var maxActiveWorkers int32

	execute := func(code string) (ExecutionResult, error) {
		current := atomic.AddInt32(&activeWorkers, 1)

		// Record the maximum number of simultaneous executions.
		for {
			maximum := atomic.LoadInt32(&maxActiveWorkers)

			if current <= maximum {
				break
			}

			if atomic.CompareAndSwapInt32(
				&maxActiveWorkers,
				maximum,
				current,
			) {
				break
			}
		}

		time.Sleep(300 * time.Millisecond)

		atomic.AddInt32(&activeWorkers, -1)

		return ExecutionResult{
			Stdout:   code,
			ExitCode: 0,
		}, nil
	}

	for i := 1; i <= 3; i++ {
		job := Job{
			ID:     fmt.Sprintf("concurrent-job-%d", i),
			Code:   fmt.Sprintf("job-%d", i),
			Status: "queued",
		}

		if err := manager.Submit(job); err != nil {
			t.Fatal(err)
		}
	}

	manager.StartWorkers(3, execute)

	for i := 1; i <= 3; i++ {
		waitForStatus(
			t,
			manager,
			fmt.Sprintf("concurrent-job-%d", i),
			"completed",
		)
	}

	if got := atomic.LoadInt32(&maxActiveWorkers); got != 3 {
		t.Fatalf("expected 3 concurrent workers, got %d", got)
	}
}
