package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Srinu0725/algotrace/internal/executor"
)

// --------------------------------------------------
// EXECUTION MODES
// --------------------------------------------------

const (
	ModeRun   = "run"
	ModeTrace = "trace"
)

// --------------------------------------------------
// JOB STATUSES
// --------------------------------------------------

const (
	StatusQueued       = "queued"
	StatusRunning      = "running"
	StatusCompleted    = "completed"
	StatusFailed       = "failed"
	StatusTimedOut     = "timed_out"
	StatusLimitReached = "limit_reached"
)

// --------------------------------------------------
// TRACE STORAGE LIMITS
// --------------------------------------------------

const (
	maxStoredTraceBytes = 64 * 1024 * 1024
	maxStoredTraces     = 100
)

// --------------------------------------------------
// JOB MODEL
// --------------------------------------------------

type Job struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	Mode            string    `json:"mode"`
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

	// Trace metadata.
	TraceAvailable bool   `json:"trace_available"`
	TraceStatus    string `json:"trace_status,omitempty"`
	TotalSteps     int    `json:"total_steps"`
	TraceSizeBytes int    `json:"trace_size_bytes"`
	TimedOut       bool   `json:"timed_out"`
}

// --------------------------------------------------
// EXECUTION RESULT
// --------------------------------------------------

type ExecutionResult struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	DurationMS      int64
	StdoutTruncated bool
	StderrTruncated bool

	TimedOut bool
	Trace    *executor.TraceResult
}

// ExecutionFunc preserves the existing executor interface.
type ExecutionFunc func(string) (ExecutionResult, error)

// JobExecutionFunc supports execution based on job mode.
type JobExecutionFunc func(Job) (ExecutionResult, error)

// --------------------------------------------------
// MANAGER
// --------------------------------------------------

type Manager struct {
	mu             sync.RWMutex
	store          map[string]Job
	queue          chan string
	closed         bool
	workersStarted bool

	traceEnabled bool

	// Serialized trace storage.
	traces     map[string][]byte
	traceOrder []string
	traceBytes int

	// Job event subscriptions.
	subscribers     map[string]map[chan JobEvent]struct{}
	subscriberCount int

	workerWG     sync.WaitGroup
	shutdownOnce sync.Once
	workersDone  chan struct{}
}

// --------------------------------------------------
// CONSTRUCTOR
// --------------------------------------------------

func NewManager(queueSize int) *Manager {
	return &Manager{
		store:       make(map[string]Job),
		traces:      make(map[string][]byte),
		queue:       make(chan string, queueSize),
		workersDone: make(chan struct{}),

		subscribers: make(
			map[string]map[chan JobEvent]struct{},
		),
	}
}

// --------------------------------------------------
// JOB SUBMISSION
// --------------------------------------------------

func (m *Manager) Submit(j Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New("job manager is shutting down")
	}

	if j.ID == "" {
		return errors.New("job ID cannot be empty")
	}

	if _, exists := m.store[j.ID]; exists {
		return errors.New("job ID already exists")
	}

	// Preserve backward compatibility.
	if j.Mode == "" {
		j.Mode = ModeRun
	}

	switch j.Mode {
	case ModeRun, ModeTrace:
		// Supported modes.

	default:
		return fmt.Errorf(
			"unsupported execution mode: %q",
			j.Mode,
		)
	}

	if j.Mode == ModeTrace &&
		m.workersStarted &&
		!m.traceEnabled {

		return errors.New(
			"trace execution is not enabled",
		)
	}

	now := time.Now()

	// Only accept submission-related fields.
	// Execution metadata is initialized by the manager.
	j = Job{
		ID:        j.ID,
		Code:      j.Code,
		Mode:      j.Mode,
		Status:    StatusQueued,
		CreatedAt: j.CreatedAt,
		UpdatedAt: now,
	}

	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
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

// --------------------------------------------------
// JOB RETRIEVAL
// --------------------------------------------------

func (m *Manager) Get(id string) (Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	j, ok := m.store[id]

	return j, ok
}

// GetTrace returns a newly decoded trace.
//
// Callers receive independent maps, slices and pointers.
// Modifying the returned trace cannot mutate manager storage.
func (m *Manager) GetTrace(
	id string,
) (*executor.TraceResult, bool) {

	m.mu.RLock()

	data, exists := m.traces[id]

	m.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Stored byte slices are immutable after insertion.
	var trace executor.TraceResult

	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, false
	}

	return &trace, true
}

// GetTraceJSON returns an independent copy of the stored JSON.
//
// This can be used by the future HTTP trace endpoint
// without decoding and encoding the trace again.
func (m *Manager) GetTraceJSON(id string) ([]byte, bool) {
	m.mu.RLock()

	data, exists := m.traces[id]

	m.mu.RUnlock()

	if !exists {
		return nil, false
	}

	return append([]byte(nil), data...), true
}

// --------------------------------------------------
// JOB UPDATES
// --------------------------------------------------

func (m *Manager) updateJob(
	id string,
	update func(*Job),
) {

	m.mu.Lock()
	defer m.mu.Unlock()

	j, ok := m.store[id]

	if !ok {
		return
	}

	update(&j)

	j.UpdatedAt = time.Now()

	m.store[id] = j

	// Notify subscribers about the updated state.
	m.publishJobLocked(j)
}

// --------------------------------------------------
// BOUNDED TRACE STORAGE
// --------------------------------------------------

// storeTraceLocked must be called while holding m.mu.
//
// Traces are evicted in insertion order when either
// the byte limit or the trace-count limit is reached.
func (m *Manager) storeTraceLocked(
	id string,
	data []byte,
) bool {

	if len(data) == 0 ||
		len(data) > maxStoredTraceBytes {

		return false
	}

	if _, exists := m.traces[id]; exists {
		return false
	}

	for len(m.traceOrder) > 0 &&
		(m.traceBytes+len(data) > maxStoredTraceBytes ||
			len(m.traces) >= maxStoredTraces) {

		oldestID := m.traceOrder[0]

		m.traceOrder[0] = ""
		m.traceOrder = m.traceOrder[1:]

		oldestTrace := m.traces[oldestID]

		m.traceBytes -= len(oldestTrace)

		delete(m.traces, oldestID)

		// Preserve the job summary even after its
		// detailed trace has been evicted.
		if job, exists := m.store[oldestID]; exists {
			job.TraceAvailable = false
			job.UpdatedAt = time.Now()

			m.store[oldestID] = job
		}
	}

	m.traces[id] = data

	m.traceOrder = append(m.traceOrder, id)

	m.traceBytes += len(data)

	return true
}

// --------------------------------------------------
// WORKER STARTUP
// --------------------------------------------------

// StartWorker preserves compatibility with the original API.
func (m *Manager) StartWorker(
	execute func(string) (ExecutionResult, error),
) error {

	return m.StartWorkers(1, execute)
}

// StartWorkers preserves existing raw execution behavior.
func (m *Manager) StartWorkers(
	count int,
	execute func(string) (ExecutionResult, error),
) error {

	if execute == nil {
		return errors.New(
			"execution function cannot be nil",
		)
	}

	return m.startWorkers(
		count,
		func(job Job) (ExecutionResult, error) {

			if job.Mode != ModeRun {
				return ExecutionResult{}, errors.New(
					"trace execution is not enabled",
				)
			}

			return execute(job.Code)
		},
		false,
	)
}

// StartWorkersWithTrace supports both execution modes.
func (m *Manager) StartWorkersWithTrace(
	count int,
	run ExecutionFunc,
	trace ExecutionFunc,
) error {

	if run == nil {
		return errors.New(
			"raw execution function cannot be nil",
		)
	}

	if trace == nil {
		return errors.New(
			"trace execution function cannot be nil",
		)
	}

	return m.startWorkers(
		count,
		func(job Job) (ExecutionResult, error) {

			switch job.Mode {

			case ModeRun:
				return run(job.Code)

			case ModeTrace:
				return trace(job.Code)

			default:
				return ExecutionResult{}, fmt.Errorf(
					"unsupported execution mode: %q",
					job.Mode,
				)
			}
		},
		true,
	)
}

// Internal worker startup shared by both public methods.
func (m *Manager) startWorkers(
	count int,
	execute JobExecutionFunc,
	traceEnabled bool,
) error {

	if count <= 0 {
		return errors.New(
			"worker count must be greater than zero",
		)
	}

	if execute == nil {
		return errors.New(
			"execution function cannot be nil",
		)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errors.New(
			"job manager is shutting down",
		)
	}

	if m.workersStarted {
		return errors.New(
			"workers have already been started",
		)
	}

	m.workersStarted = true
	m.traceEnabled = traceEnabled

	m.workerWG.Add(count)

	for i := 0; i < count; i++ {
		go m.worker(execute)
	}

	return nil
}

// --------------------------------------------------
// WORKER EXECUTION
// --------------------------------------------------

func (m *Manager) worker(execute JobExecutionFunc) {
	defer m.workerWG.Done()

	for id := range m.queue {

		job, ok := m.Get(id)

		if !ok {
			continue
		}

		m.updateJob(id, func(j *Job) {
			j.Status = StatusRunning
		})

		result, err := executeJobSafely(execute, job)

		m.finishJob(id, result, err)
	}
}

// --------------------------------------------------
// FINALIZE EXECUTION
// --------------------------------------------------

func (m *Manager) finishJob(
	id string,
	result ExecutionResult,
	executionErr error,
) {

	var traceData []byte

	// Serialize outside the manager lock.
	if result.Trace != nil {

		data, err := json.Marshal(result.Trace)

		if err != nil {
			executionErr = errors.Join(
				executionErr,
				fmt.Errorf(
					"could not serialize trace: %w",
					err,
				),
			)
		} else {
			traceData = data
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.store[id]

	if !exists {
		return
	}

	job.Stdout = result.Stdout
	job.Stderr = result.Stderr
	job.ExitCode = result.ExitCode
	job.DurationMS = result.DurationMS
	job.StdoutTruncated = result.StdoutTruncated
	job.StderrTruncated = result.StderrTruncated
	job.TimedOut = result.TimedOut
	job.Error = ""

	// Store valid trace information even if the
	// submitted Python program failed.
	if result.Trace != nil {

		job.TraceStatus = result.Trace.Status
		job.TotalSteps = result.Trace.TotalSteps
		job.TraceSizeBytes = result.Trace.TraceSizeBytes

		job.Stdout = result.Trace.Stdout
		job.Stderr = result.Trace.Stderr

		job.StdoutTruncated =
			result.Trace.StdoutTruncated

		job.StderrTruncated =
			result.Trace.StderrTruncated

		job.TraceAvailable = m.storeTraceLocked(
			id,
			traceData,
		)

		if len(traceData) > 0 &&
			!job.TraceAvailable {

			executionErr = errors.Join(
				executionErr,
				errors.New("trace storage limit exceeded"),
			)
		}
	}

	// Infrastructure failures have priority.
	switch {

	case executionErr != nil:

		job.Status = StatusFailed
		job.Error = executionErr.Error()

	case result.TimedOut:

		job.Status = StatusTimedOut
		job.ExitCode = -1
		job.Error = "Execution timed out"

	case result.Trace != nil:

		switch result.Trace.Status {

		case "completed":
			job.Status = StatusCompleted

		case "failed":
			job.Status = StatusFailed

			if result.Trace.Error != nil {
				job.Error = fmt.Sprintf(
					"%s: %s",
					result.Trace.Error.Type,
					result.Trace.Error.Message,
				)
			}

		case "limit_reached":
			job.Status = StatusLimitReached

		default:
			job.Status = StatusFailed
			job.Error = "Unknown trace status"
		}

	case job.Mode == ModeTrace:

		job.Status = StatusFailed
		job.Error = "Trace execution returned no trace"

	case result.ExitCode != 0:

		job.Status = StatusFailed

	default:

		job.Status = StatusCompleted
	}

	job.UpdatedAt = time.Now()

	m.store[id] = job

	// Publish the final execution result.
	m.publishJobLocked(job)
}

// --------------------------------------------------
// PANIC RECOVERY
// --------------------------------------------------

// Preserve the original helper.
func executeSafely(
	execute func(string) (ExecutionResult, error),
	code string,
) (result ExecutionResult, err error) {

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf(
				"execution panicked: %v",
				recovered,
			)
		}
	}()

	return execute(code)
}

// Job-aware panic recovery.
func executeJobSafely(
	execute JobExecutionFunc,
	job Job,
) (result ExecutionResult, err error) {

	return executeSafely(
		func(string) (ExecutionResult, error) {
			return execute(job)
		},
		job.Code,
	)
}

// --------------------------------------------------
// GRACEFUL SHUTDOWN
// -----------func (m *Manager) Shutdown(ctx context.Context) error {
func (m *Manager) Shutdown(ctx context.Context) error {

	m.shutdownOnce.Do(func() {

		m.mu.Lock()

		m.closed = true

		close(m.queue)

		m.mu.Unlock()

		go func() {

			// Allow workers to complete queued jobs
			// and publish their terminal events.
			m.workerWG.Wait()

			m.mu.Lock()

			// Release any remaining subscriptions.
			m.closeSubscribersLocked()

			m.mu.Unlock()

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
