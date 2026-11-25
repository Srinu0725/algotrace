package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	Status          string    `json:"status"`
	Stdout          string    `json:"stdout"`
	Stderr          string    `json:"stderr"`
	ExitCode        int       `json:"exit_code"`
	DurationMS      int64     `json:"duration_ms"`
	StdoutTruncated bool      `json:"stdout_truncated"`
	StderrTruncated bool      `json:"stderr_truncated"`
	Error           string    `json:"error,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ExecutionResult struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	DurationMS      int64
	StdoutTruncated bool
	StderrTruncated bool
}

type Manager struct {
	mu             sync.RWMutex
	store          map[string]Job
	queue          chan string
	closed         bool
	workersStarted bool

	workerWG     sync.WaitGroup
	shutdownOnce sync.Once
	workersDone  chan struct{}
}

func NewManager(queueSize int) *Manager {
	return &Manager{
		store:       make(map[string]Job),
		queue:       make(chan string, queueSize),
		workersDone: make(chan struct{}),
	}
}

// Submit adds a job to the queue.
// New submissions are rejected once shutdown begins.
func (m *Manager) Submit(j Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("job manager is shutting down")
	}

	m.store[j.ID] = j

	select {
	case m.queue <- j.ID:
		return nil

	default:
		delete(m.store, j.ID)
		return errors.New("job queue is full")
	}
}

func (m *Manager) Get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	j, ok := m.store[id]
	return j, ok
}

func (m *Manager) updateJob(id string, update func(*Job)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.store[id]
	if !ok {
		return
	}

	update(&j)
	j.UpdatedAt = time.Now()
	m.store[id] = j
}

// StartWorker preserves compatibility with the original API.
func (m *Manager) StartWorker(
	execute func(string) (ExecutionResult, error),
) error {
	return m.StartWorkers(1, execute)
}

// StartWorkers launches a configurable number of workers.
// Workers can only be started once.
func (m *Manager) StartWorkers(
	count int,
	execute func(string) (ExecutionResult, error),
) error {
	if count <= 0 {
		return errors.New("worker count must be greater than zero")
	}

	if execute == nil {
		return errors.New("execution function cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("job manager is shutting down")
	}

	if m.workersStarted {
		return errors.New("workers have already been started")
	}

	m.workersStarted = true
	m.workerWG.Add(count)

	for i := 0; i < count; i++ {
		go m.worker(execute)
	}

	return nil
}

// worker continuously processes jobs until the queue is closed
// and all queued jobs have been consumed.
func (m *Manager) worker(
	execute func(string) (ExecutionResult, error),
) {
	defer m.workerWG.Done()

	for id := range m.queue {
		job, ok := m.Get(id)
		if !ok {
			continue
		}

		m.updateJob(id, func(j *Job) {
			j.Status = "running"
		})

		result, err := executeSafely(execute, job.Code)

		m.updateJob(id, func(j *Job) {
			j.Stdout = result.Stdout
			j.Stderr = result.Stderr
			j.ExitCode = result.ExitCode
			j.DurationMS = result.DurationMS
			j.StdoutTruncated = result.StdoutTruncated
			j.StderrTruncated = result.StderrTruncated

			if err != nil {
				j.Status = "failed"
				j.Error = err.Error()
				return
			}

			if result.ExitCode != 0 {
				j.Status = "failed"
				return
			}

			j.Status = "completed"
		})
	}
}

// executeSafely prevents a panic in one execution from killing a worker.
func executeSafely(
	execute func(string) (ExecutionResult, error),
	code string,
) (result ExecutionResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("execution panicked: %v", recovered)
		}
	}()

	return execute(code)
}

// Shutdown stops accepting new jobs, closes the queue,
// and waits for all workers to finish processing queued jobs.
//
// If the context deadline expires, Shutdown returns the context error.
// Workers may continue running until their execution functions return.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.shutdownOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		close(m.queue)
		m.mu.Unlock()

		go func() {
			m.workerWG.Wait()
			close(m.workersDone)
		}()
	})

	select {
	case <-m.workersDone:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
