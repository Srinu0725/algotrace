package executor

import "testing"

func TestDecodeTraceResponseSuccess(t *testing.T) {
	output := `{
		"status": "completed",
		"limit": null,
		"error": null,
		"stdout": "30\n",
		"stderr": "",
		"total_steps": 1,
		"trace_size_bytes": 100,
		"events": [
			{
				"step": 1,
				"line": 3,
				"event": "line",
				"function": "<module>",
				"depth": 0,
				"variables": {
					"result": 30
				}
			}
		]
	}`

	trace, err := decodeTraceResponse(output)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.Status != "completed" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if trace.Stdout != "30\n" {
		t.Fatalf("unexpected stdout: %q", trace.Stdout)
	}

	if len(trace.Events) != 1 {
		t.Fatal("expected one trace event")
	}

	value := trace.Events[0].Variables["result"]

	if string(value) != "30" {
		t.Fatalf("unexpected variable value: %s", value)
	}
}

func TestDecodeTraceResponseRuntimeError(t *testing.T) {
	output := `{
		"status": "failed",
		"limit": null,
		"error": {
			"type": "ZeroDivisionError",
			"message": "division by zero",
			"line": 3,
			"function": "<module>"
		},
		"stdout": "",
		"stderr": "",
		"total_steps": 0,
		"trace_size_bytes": 0,
		"events": []
	}`

	trace, err := decodeTraceResponse(output)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.Status != "failed" {
		t.Fatal("expected failed execution")
	}

	if trace.Error == nil {
		t.Fatal("expected structured error")
	}

	if trace.Error.Type != "ZeroDivisionError" {
		t.Fatalf(
			"unexpected error type: %s",
			trace.Error.Type,
		)
	}
}

func TestDecodeTraceResponseLimitReached(t *testing.T) {
	output := `{
		"status": "limit_reached",
		"limit": "max_steps",
		"error": null,
		"total_steps": 0,
		"trace_size_bytes": 0,
		"events": []
	}`

	trace, err := decodeTraceResponse(output)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trace.Status != "limit_reached" {
		t.Fatal("expected limit_reached status")
	}

	if trace.Limit == nil {
		t.Fatal("expected limit information")
	}
}

func TestDecodeTraceResponseInvalid(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "empty response",
			output: "",
		},
		{
			name:   "invalid JSON",
			output: `{"status":`,
		},
		{
			name:   "unknown status",
			output: `{"status":"unknown"}`,
		},
		{
			name: "negative steps",
			output: `{
				"status":"completed",
				"total_steps":-1
			}`,
		},
		{
			name: "step count mismatch",
			output: `{
				"status":"completed",
				"total_steps":5,
				"events":[]
			}`,
		},
		{
			name: "missing error",
			output: `{
				"status":"failed",
				"events":[]
			}`,
		},
		{
			name: "missing limit",
			output: `{
				"status":"limit_reached",
				"events":[]
			}`,
		},
		{
			name: "negative trace size",
			output: `{
				"status":"completed",
				"trace_size_bytes":-1
			}`,
		},
		{
			name: "multiple JSON objects",
			output: `{"status":"completed"}
			         {"status":"completed"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeTraceResponse(tt.output)

			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
