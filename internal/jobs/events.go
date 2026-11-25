package jobs

import (
	"errors"
	"sync"
	"time"
)

const (
	EventJobUpdated = "job_updated"

	maxSubscribersPerJob = 32
	maxTotalSubscribers  = 1024
)

var (
	ErrJobNotFound = errors.New("job not found")

	ErrManagerClosed = errors.New(
		"job manager is shutting down",
	)

	ErrSubscriberLimit = errors.New(
		"subscriber limit reached",
	)
)

// JobEvent contains lightweight execution metadata.
// Source code, stdout, stderr and complete traces
// are intentionally excluded.
type JobEvent struct {
	Type           string    `json:"type"`
	JobID          string    `json:"job_id"`
	Mode           string    `json:"mode"`
	Status         string    `json:"status"`
	TraceAvailable bool      `json:"trace_available"`
	TraceStatus    string    `json:"trace_status,omitempty"`
	TotalSteps     int       `json:"total_steps"`
	DurationMS     int64     `json:"duration_ms"`
	ExitCode       int       `json:"exit_code"`
	TimedOut       bool      `json:"timed_out"`
	Error          string    `json:"error,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// IsTerminalStatus reports whether execution has finished.
func IsTerminalStatus(status string) bool {
	switch status {

	case StatusCompleted,
		StatusFailed,
		StatusTimedOut,
		StatusLimitReached:

		return true

	default:
		return false
	}
}

// newJobEvent creates an independent metadata snapshot.
func newJobEvent(job Job) JobEvent {
	return JobEvent{
		Type:           EventJobUpdated,
		JobID:          job.ID,
		Mode:           job.Mode,
		Status:         job.Status,
		TraceAvailable: job.TraceAvailable,
		TraceStatus:    job.TraceStatus,
		TotalSteps:     job.TotalSteps,
		DurationMS:     job.DurationMS,
		ExitCode:       job.ExitCode,
		TimedOut:       job.TimedOut,
		Error:          job.Error,
		UpdatedAt:      job.UpdatedAt,
	}
}

// Subscribe registers a subscriber for a specific job.
//
// The initial snapshot and subscription registration
// happen under the same lock to prevent missed updates.
//
// The returned cancellation function is safe to call
// multiple times and from multiple goroutines.
func (m *Manager) Subscribe(
	jobID string,
) (<-chan JobEvent, func(), error) {

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, nil, ErrManagerClosed
	}

	job, exists := m.store[jobID]

	if !exists {
		return nil, nil, ErrJobNotFound
	}

	// A single-slot channel stores the latest state.
	ch := make(chan JobEvent, 1)

	// Completed jobs do not require a persistent
	// subscription.
	if IsTerminalStatus(job.Status) {

		ch <- newJobEvent(job)

		close(ch)

		return ch, func() {}, nil
	}

	if m.subscriberCount >= maxTotalSubscribers {
		return nil, nil, ErrSubscriberLimit
	}

	if len(m.subscribers[jobID]) >= maxSubscribersPerJob {
		return nil, nil, ErrSubscriberLimit
	}

	if m.subscribers[jobID] == nil {
		m.subscribers[jobID] =
			make(map[chan JobEvent]struct{})
	}

	m.subscribers[jobID][ch] = struct{}{}

	m.subscriberCount++

	// Send the current job state immediately.
	ch <- newJobEvent(job)

	var once sync.Once

	cancel := func() {

		once.Do(func() {

			m.mu.Lock()
			defer m.mu.Unlock()

			m.removeSubscriberLocked(jobID, ch)
		})
	}

	return ch, cancel, nil
}

// removeSubscriberLocked removes and closes a subscriber.
//
// The caller must hold m.mu.
func (m *Manager) removeSubscriberLocked(
	jobID string,
	ch chan JobEvent,
) {

	subscribers, exists := m.subscribers[jobID]

	if !exists {
		return
	}

	if _, exists := subscribers[ch]; !exists {
		return
	}

	delete(subscribers, ch)

	m.subscriberCount--

	close(ch)

	if len(subscribers) == 0 {
		delete(m.subscribers, jobID)
	}
}

// offerLatest replaces an unread event with the latest
// job state without blocking the execution worker.
//
// Only the manager may send to this channel.
func offerLatest(
	ch chan JobEvent,
	event JobEvent,
) {

	select {

	case ch <- event:
		return

	default:
		// Subscriber has not consumed the previous event.
	}

	// Remove the outdated event if it is still present.
	select {

	case <-ch:

	default:
	}

	// The manager serializes all publishers, so there
	// is now room for the latest event.
	ch <- event
}

// publishJobLocked broadcasts a job snapshot.
//
// The caller must hold m.mu.
//
// Terminal events are delivered before the subscriber
// channels are closed.
func (m *Manager) publishJobLocked(job Job) {

	subscribers := m.subscribers[job.ID]

	if len(subscribers) == 0 {
		return
	}

	event := newJobEvent(job)

	terminal := IsTerminalStatus(job.Status)

	for ch := range subscribers {

		offerLatest(ch, event)

		if terminal {
			m.removeSubscriberLocked(job.ID, ch)
		}
	}
}

// closeSubscribersLocked releases all remaining
// subscriptions during manager shutdown.
//
// The caller must hold m.mu.
func (m *Manager) closeSubscribersLocked() {

	for jobID, subscribers := range m.subscribers {

		for ch := range subscribers {

			m.removeSubscriberLocked(jobID, ch)
		}
	}
}
