package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newEventTestManager(t *testing.T) *Manager {

	t.Helper()

	manager := NewManager(100)

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

func submitEventTestJob(
	t *testing.T,
	manager *Manager,
	id string,
) {

	t.Helper()

	err := manager.Submit(Job{
		ID:   id,
		Code: "print(30)",
		Mode: ModeRun,
	})

	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
}

func receiveJobEvent(
	t *testing.T,
	ch <-chan JobEvent,
) JobEvent {

	t.Helper()

	select {

	case event, ok := <-ch:

		if !ok {
			t.Fatal("subscription closed unexpectedly")
		}

		return event

	case <-time.After(3 * time.Second):

		t.Fatal("timed out waiting for job event")

		return JobEvent{}
	}
}

func assertSubscriptionClosed(
	t *testing.T,
	ch <-chan JobEvent,
) {

	t.Helper()

	select {

	case _, ok := <-ch:

		if ok {
			t.Fatal("expected closed subscription")
		}

	case <-time.After(3 * time.Second):

		t.Fatal("subscription was not closed")
	}
}

func TestSubscribeInitialSnapshot(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel()

	event := receiveJobEvent(t, ch)

	if event.JobID != "job-1" {
		t.Fatalf("unexpected job ID: %s", event.JobID)
	}

	if event.Status != StatusQueued {
		t.Fatalf("unexpected status: %s", event.Status)
	}

	if event.Type != EventJobUpdated {
		t.Fatalf("unexpected event type: %s", event.Type)
	}
}

func TestSubscribeUnknownJob(t *testing.T) {

	manager := newEventTestManager(t)

	_, _, err := manager.Subscribe("missing-job")

	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestSubscribeJobLifecycle(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel()

	initial := receiveJobEvent(t, ch)

	if initial.Status != StatusQueued {
		t.Fatal("expected queued status")
	}

	release := make(chan struct{})

	var releaseOnce sync.Once

	unblock := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}

	defer unblock()

	err = manager.StartWorker(
		func(string) (ExecutionResult, error) {

			<-release

			return ExecutionResult{
				Stdout:     "30\n",
				ExitCode:   0,
				DurationMS: 100,
			}, nil
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	running := receiveJobEvent(t, ch)

	if running.Status != StatusRunning {
		t.Fatalf("expected running, got %s", running.Status)
	}

	unblock()

	completed := receiveJobEvent(t, ch)

	if completed.Status != StatusCompleted {
		t.Fatalf(
			"expected completed, got %s",
			completed.Status,
		)
	}

	if completed.DurationMS != 100 {
		t.Fatal("incorrect execution metadata")
	}

	assertSubscriptionClosed(t, ch)
}

func TestMultipleJobSubscribers(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	const count = 5

	channels := make([]<-chan JobEvent, 0, count)

	for i := 0; i < count; i++ {

		ch, cancel, err := manager.Subscribe("job-1")

		if err != nil {
			t.Fatal(err)
		}

		defer cancel()

		receiveJobEvent(t, ch)

		channels = append(channels, ch)
	}

	manager.updateJob("job-1", func(job *Job) {
		job.Status = StatusRunning
	})

	for _, ch := range channels {

		event := receiveJobEvent(t, ch)

		if event.Status != StatusRunning {
			t.Fatal("subscriber did not receive running state")
		}
	}

	manager.finishJob(
		"job-1",
		ExecutionResult{ExitCode: 0},
		nil,
	)

	for _, ch := range channels {

		event := receiveJobEvent(t, ch)

		if event.Status != StatusCompleted {
			t.Fatal("subscriber missed completion")
		}

		assertSubscriptionClosed(t, ch)
	}
}

func TestJobSubscriptionIsolation(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")
	submitEventTestJob(t, manager, "job-2")

	ch1, cancel1, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel1()

	ch2, cancel2, err := manager.Subscribe("job-2")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel2()

	receiveJobEvent(t, ch1)
	receiveJobEvent(t, ch2)

	manager.updateJob("job-1", func(job *Job) {
		job.Status = StatusRunning
	})

	event := receiveJobEvent(t, ch1)

	if event.JobID != "job-1" {
		t.Fatal("incorrect job event")
	}

	select {

	case unexpected := <-ch2:

		t.Fatalf(
			"received another job's event: %+v",
			unexpected,
		)

	default:
	}
}

func TestSlowSubscriberReceivesLatestState(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel()

	// Intentionally leave the initial event unread.
	manager.updateJob("job-1", func(job *Job) {
		job.Status = StatusRunning
	})

	manager.finishJob(
		"job-1",
		ExecutionResult{ExitCode: 0},
		nil,
	)

	event := receiveJobEvent(t, ch)

	if event.Status != StatusCompleted {
		t.Fatalf(
			"expected latest terminal state, got %s",
			event.Status,
		)
	}

	assertSubscriptionClosed(t, ch)
}

func TestSubscribeCompletedJob(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	manager.finishJob(
		"job-1",
		ExecutionResult{ExitCode: 0},
		nil,
	)

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel()

	event := receiveJobEvent(t, ch)

	if event.Status != StatusCompleted {
		t.Fatal("expected completed snapshot")
	}

	assertSubscriptionClosed(t, ch)

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	if manager.subscriberCount != 0 {
		t.Fatal("terminal subscription was retained")
	}
}

func TestSubscriptionCancellation(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	receiveJobEvent(t, ch)

	cancel()
	cancel()

	assertSubscriptionClosed(t, ch)

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	if manager.subscriberCount != 0 {
		t.Fatal("subscriber was not removed")
	}

	if len(manager.subscribers) != 0 {
		t.Fatal("empty subscriber group was retained")
	}
}

func TestConcurrentSubscriptionCancellation(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	receiveJobEvent(t, ch)

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {

		wg.Add(1)

		go func() {

			defer wg.Done()

			cancel()
		}()
	}

	wg.Wait()

	assertSubscriptionClosed(t, ch)
}

func TestConcurrentPublishAndCancel(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {

		_, cancel, err := manager.Subscribe("job-1")

		if err != nil {
			t.Fatal(err)
		}

		wg.Add(1)

		go func(cancel func()) {

			defer wg.Done()

			cancel()

		}(cancel)
	}

	wg.Add(1)

	go func() {

		defer wg.Done()

		manager.updateJob("job-1", func(job *Job) {
			job.Status = StatusRunning
		})

		manager.finishJob(
			"job-1",
			ExecutionResult{ExitCode: 0},
			nil,
		)
	}()

	wg.Wait()

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	if manager.subscriberCount != 0 {
		t.Fatal("subscriber leak detected")
	}
}

func TestConcurrentSubscribeAndFinish(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	var wg sync.WaitGroup

	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {

		wg.Add(1)

		go func() {

			defer wg.Done()

			ch, cancel, err := manager.Subscribe("job-1")

			if err != nil {
				errs <- err
				return
			}

			defer cancel()

			timeout := time.NewTimer(3 * time.Second)

			defer timeout.Stop()

			for {

				select {

				case event, ok := <-ch:

					if !ok {
						errs <- errors.New(
							"closed before terminal event",
						)
						return
					}

					if event.Status == StatusCompleted {
						return
					}

				case <-timeout.C:

					errs <- errors.New(
						"terminal event was missed",
					)

					return
				}
			}
		}()
	}

	manager.finishJob(
		"job-1",
		ExecutionResult{ExitCode: 0},
		nil,
	)

	wg.Wait()

	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestSubscriberLimit(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	for i := 0; i < maxSubscribersPerJob; i++ {

		_, cancel, err := manager.Subscribe("job-1")

		if err != nil {
			t.Fatal(err)
		}

		defer cancel()
	}

	_, _, err := manager.Subscribe("job-1")

	if !errors.Is(err, ErrSubscriberLimit) {
		t.Fatalf("expected subscriber limit, got %v", err)
	}
}

func TestGlobalSubscriberLimit(t *testing.T) {

	manager := newEventTestManager(t)

	for i := 0; i < maxTotalSubscribers; i++ {

		jobID := fmt.Sprintf(
			"job-%d",
			i/maxSubscribersPerJob,
		)

		if i%maxSubscribersPerJob == 0 {
			submitEventTestJob(t, manager, jobID)
		}

		_, cancel, err := manager.Subscribe(jobID)

		if err != nil {
			t.Fatal(err)
		}

		defer cancel()
	}

	submitEventTestJob(t, manager, "extra-job")

	_, _, err := manager.Subscribe("extra-job")

	if !errors.Is(err, ErrSubscriberLimit) {
		t.Fatalf(
			"expected global subscriber limit, got %v",
			err,
		)
	}
}

func TestShutdownClosesSubscriptions(t *testing.T) {

	manager := newEventTestManager(t)

	submitEventTestJob(t, manager, "job-1")

	ch, cancel, err := manager.Subscribe("job-1")

	if err != nil {
		t.Fatal(err)
	}

	defer cancel()

	receiveJobEvent(t, ch)

	ctx, stop := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)

	defer stop()

	if err := manager.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	assertSubscriptionClosed(t, ch)

	_, _, err = manager.Subscribe("job-1")

	if !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("expected manager closed error, got %v", err)
	}
}

func TestTerminalJobStatuses(t *testing.T) {

	tests := []struct {
		name   string
		result ExecutionResult
		err    error
		status string
	}{
		{
			name:   "completed",
			result: ExecutionResult{ExitCode: 0},
			status: StatusCompleted,
		},
		{
			name:   "failed",
			result: ExecutionResult{ExitCode: 1},
			status: StatusFailed,
		},
		{
			name: "timeout",
			result: ExecutionResult{
				TimedOut: true,
			},
			status: StatusTimedOut,
		},
		{
			name:   "infrastructure failure",
			err:    errors.New("executor unavailable"),
			status: StatusFailed,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			manager := newEventTestManager(t)

			submitEventTestJob(t, manager, "job-1")

			ch, cancel, err := manager.Subscribe("job-1")

			if err != nil {
				t.Fatal(err)
			}

			defer cancel()

			receiveJobEvent(t, ch)

			manager.finishJob(
				"job-1",
				tt.result,
				tt.err,
			)

			event := receiveJobEvent(t, ch)

			if event.Status != tt.status {
				t.Fatalf(
					"expected %s, got %s",
					tt.status,
					event.Status,
				)
			}

			assertSubscriptionClosed(t, ch)
		})
	}
}
