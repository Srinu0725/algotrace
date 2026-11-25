//go:build integration

package executor

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Docker availability is checked by each integration test.
	m.Run()
}

func requireDocker(t *testing.T) {
	t.Helper()

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatal("Docker must be running for integration tests")
	}

	if err := exec.Command(
		"docker",
		"image",
		"inspect",
		tracerImage,
	).Run(); err != nil {
		t.Fatalf("Required Docker image not found: %s", tracerImage)
	}
}

func executeTrace(t *testing.T, code string) *TraceResult {
	t.Helper()

	requireDocker(t)

	result, err := RunPythonTrace(code)

	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if result.TimedOut {
		t.Fatal("unexpected execution timeout")
	}

	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	if result.Trace == nil {
		t.Fatal("expected a structured trace")
	}

	if result.Trace.TotalSteps != len(result.Trace.Events) {
		t.Fatal("trace step count mismatch")
	}

	return result.Trace
}

// --------------------------------------------------
// 1. NORMAL EXECUTION
// --------------------------------------------------

func TestRunPythonTraceBasic(t *testing.T) {
	code := `
x = 10
y = 20
result = x + y
print(result)
`

	trace := executeTrace(t, code)

	if trace.Status != "completed" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if strings.TrimSpace(trace.Stdout) != "30" {
		t.Fatalf("unexpected stdout: %q", trace.Stdout)
	}

	if trace.Error != nil {
		t.Fatalf("unexpected execution error: %+v", trace.Error)
	}

	if trace.TotalSteps == 0 {
		t.Fatal("expected execution events")
	}

	found := false

	for _, event := range trace.Events {
		if string(event.Variables["result"]) == "30" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("result variable was not captured")
	}
}

// --------------------------------------------------
// 2. ARRAY MUTATION
// --------------------------------------------------

func TestRunPythonTraceArrayMutation(t *testing.T) {
	code := `
arr = [10, 20, 30]
arr.append(40)
arr[1] = 99
print(arr)
`

	trace := executeTrace(t, code)

	if trace.Status != "completed" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if strings.TrimSpace(trace.Stdout) != "[10, 99, 30, 40]" {
		t.Fatalf("unexpected stdout: %q", trace.Stdout)
	}

	foundArray := false
	foundMutation := false

	for _, event := range trace.Events {

		if raw, exists := event.Variables["arr"]; exists {
			var arr []int

			if err := json.Unmarshal(raw, &arr); err == nil {
				if len(arr) == 4 &&
					arr[0] == 10 &&
					arr[1] == 99 &&
					arr[2] == 30 &&
					arr[3] == 40 {

					foundArray = true
				}
			}
		}

		for _, change := range event.Changes {
			if change.Type == "array_updated" &&
				change.Name == "arr" {

				foundMutation = true
			}
		}
	}

	if !foundArray {
		t.Fatal("final array state was not captured")
	}

	if !foundMutation {
		t.Fatal("array mutation event was not captured")
	}
}

// --------------------------------------------------
// 3. RECURSION
// --------------------------------------------------

func TestRunPythonTraceRecursion(t *testing.T) {
	code := `
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

result = factorial(4)
print(result)
`

	trace := executeTrace(t, code)

	if trace.Status != "completed" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if strings.TrimSpace(trace.Stdout) != "24" {
		t.Fatalf("unexpected stdout: %q", trace.Stdout)
	}

	maxDepth := 0
	functionCalls := 0
	foundReturn := false

	for _, event := range trace.Events {

		if event.Depth > maxDepth {
			maxDepth = event.Depth
		}

		if event.Event == "function_call" &&
			event.Function == "factorial" {

			functionCalls++
		}

		if event.Event == "function_return" &&
			event.Function == "factorial" &&
			string(event.ReturnValue) == "24" {

			foundReturn = true
		}
	}

	if functionCalls != 4 {
		t.Fatalf("expected 4 function calls, got %d", functionCalls)
	}

	if maxDepth != 4 {
		t.Fatalf("expected recursion depth 4, got %d", maxDepth)
	}

	if !foundReturn {
		t.Fatal("factorial return value was not captured")
	}
}

// --------------------------------------------------
// 4. RUNTIME ERROR
// --------------------------------------------------

func TestRunPythonTraceRuntimeError(t *testing.T) {
	code := "print('Starting')\nx = 10\ny = 0\nprint(x / y)"

	trace := executeTrace(t, code)

	if trace.Status != "failed" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if trace.Error == nil {
		t.Fatal("expected structured error")
	}

	if trace.Error.Type != "ZeroDivisionError" {
		t.Fatalf("unexpected error type: %s", trace.Error.Type)
	}

	if trace.Error.Line == nil || *trace.Error.Line != 4 {
		t.Fatal("expected error on line 4")
	}

	if strings.TrimSpace(trace.Stdout) != "Starting" {
		t.Fatal("output before runtime error was not preserved")
	}

	foundException := false

	for _, event := range trace.Events {
		if event.Event == "exception" &&
			event.Exception != nil &&
			event.Exception.Type == "ZeroDivisionError" {

			foundException = true
		}
	}

	if !foundException {
		t.Fatal("exception event was not captured")
	}
}

// --------------------------------------------------
// 5. SYNTAX ERROR
// --------------------------------------------------

func TestRunPythonTraceSyntaxError(t *testing.T) {
	code := "def invalid_function(\n    print('hello')"

	trace := executeTrace(t, code)

	if trace.Status != "failed" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if trace.Error == nil {
		t.Fatal("expected syntax error")
	}

	if trace.Error.Type != "SyntaxError" {
		t.Fatalf("unexpected error type: %s", trace.Error.Type)
	}

	if trace.TotalSteps != 0 {
		t.Fatal("invalid Python code should not produce execution steps")
	}
}

// --------------------------------------------------
// 6. TRACE LIMIT
// --------------------------------------------------

func TestRunPythonTraceLimit(t *testing.T) {
	code := `
while True:
    pass
`

	trace := executeTrace(t, code)

	if trace.Status != "limit_reached" {
		t.Fatalf("unexpected status: %s", trace.Status)
	}

	if trace.Limit == nil {
		t.Fatal("expected trace limit information")
	}

	if trace.TotalSteps > 10000 {
		t.Fatalf("trace exceeded maximum steps: %d", trace.TotalSteps)
	}
}

// --------------------------------------------------
// 7. EXTERNAL TIMEOUT
// --------------------------------------------------

func TestRunPythonTraceTimeout(t *testing.T) {
	requireDocker(t)

	code := `
import time
time.sleep(30)
`

	result, err := RunPythonTrace(code)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.TimedOut {
		t.Fatal("expected execution timeout")
	}

	if result.ExitCode != -1 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}

	if result.Trace != nil {
		t.Fatal("timed-out execution should not return a complete trace")
	}
}

// --------------------------------------------------
// 8. CONTAINER CLEANUP
// --------------------------------------------------

func listExecutionContainers(t *testing.T) map[string]bool {
	t.Helper()

	output, err := exec.Command(
		"docker",
		"ps",
		"-a",
		"--filter",
		"name=algotrace-",
		"--format",
		"{{.Names}}",
	).Output()

	if err != nil {
		t.Fatalf("could not inspect containers: %v", err)
	}

	containers := make(map[string]bool)

	for _, name := range strings.Fields(string(output)) {
		containers[name] = true
	}

	return containers
}

func TestRunPythonTraceContainerCleanup(t *testing.T) {
	requireDocker(t)

	before := listExecutionContainers(t)

	// Verify cleanup after successful execution.
	executeTrace(t, "print(42)")

	// Verify cleanup after external timeout.
	result, err := RunPythonTrace(
		"import time\ntime.sleep(30)",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.TimedOut {
		t.Fatal("expected timeout")
	}

	after := listExecutionContainers(t)

	for name := range after {
		if !before[name] {
			t.Errorf("execution container was not removed: %s", name)
		}
	}
}
