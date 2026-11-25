package executor

import "encoding/json"

// TraceError represents a structured Python execution error.
type TraceError struct {
	Type     string `json:"type"`
	Message  string `json:"message"`
	Line     *int   `json:"line"`
	Column   *int   `json:"column,omitempty"`
	Function string `json:"function"`
}

// VariableChange represents a change in program state.
type VariableChange struct {
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Value    json.RawMessage `json:"value,omitempty"`
	OldValue json.RawMessage `json:"old_value,omitempty"`
	NewValue json.RawMessage `json:"new_value,omitempty"`
}

// TraceException represents an exception observed during execution.
type TraceException struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// TraceEvent represents a single recorded execution step.
type TraceEvent struct {
	Step     int    `json:"step"`
	Line     int    `json:"line"`
	Event    string `json:"event"`
	Function string `json:"function"`
	Depth    int    `json:"depth"`

	CallStack []string `json:"call_stack"`

	Variables map[string]json.RawMessage `json:"variables,omitempty"`
	Arguments map[string]json.RawMessage `json:"arguments,omitempty"`

	Changes []VariableChange `json:"changes,omitempty"`

	ReturnValue json.RawMessage `json:"return_value,omitempty"`

	Exception *TraceException `json:"exception,omitempty"`
}

// TraceResult represents the complete Python runner response.
type TraceResult struct {
	Status string  `json:"status"`
	Limit  *string `json:"limit"`

	Error *TraceError `json:"error"`

	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`

	StdoutTruncated bool `json:"stdout_truncated"`
	StderrTruncated bool `json:"stderr_truncated"`

	TotalSteps     int `json:"total_steps"`
	TraceSizeBytes int `json:"trace_size_bytes"`

	Events []TraceEvent `json:"events"`
}
