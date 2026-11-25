package executor

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	executionTimeout = 5 * time.Second
	cleanupTimeout   = 5 * time.Second

	maxStdoutBytes = 1 * 1024 * 1024
	maxStderrBytes = 1 * 1024 * 1024

	// Includes trace events, program output,
	// metadata and JSON escaping overhead.
	maxTraceResponseBytes = 20 * 1024 * 1024

	pythonImage = "python:3.12-alpine"
	tracerImage = "algotrace-python:dev"
)

type Result struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	StdoutTruncated bool
	StderrTruncated bool
}

// TracedResult contains the decoded Python trace
// and Docker execution metadata.
type TracedResult struct {
	Trace           *TraceResult
	Duration        time.Duration
	ExitCode        int
	TimedOut        bool
	DockerStderr    string
	StderrTruncated bool
}

// limitedBuffer stores at most maxBytes while
// accepting all writes.
type limitedBuffer struct {
	buffer    bytes.Buffer
	maxBytes  int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.maxBytes - b.buffer.Len()

	if remaining <= 0 {
		if len(p) > 0 {
			b.truncated = true
		}

		return len(p), nil
	}

	if len(p) > remaining {
		_, err := b.buffer.Write(p[:remaining])
		b.truncated = true

		return len(p), err
	}

	return b.buffer.Write(p)
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}

func appendWithinLimit(
	current, message string,
	maxBytes int,
) (string, bool) {

	if maxBytes <= 0 {
		return "", len(current)+len(message) > 0
	}

	if len(message) >= maxBytes {
		return message[:maxBytes],
			len(current)+len(message) > maxBytes
	}

	if len(current)+len(message) <= maxBytes {
		return current + message, false
	}

	keepBytes := maxBytes - len(message)

	return current[:keepBytes] + message, true
}

// Generates a unique container name for each execution.
func newContainerName() (string, error) {
	randomBytes := make([]byte, 16)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf(
			"could not generate container name: %w",
			err,
		)
	}

	return "algotrace-" +
		hex.EncodeToString(randomBytes), nil
}

// Removes a container using an independent timeout.
func removeContainer(name string) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		cleanupTimeout,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"docker",
		"rm",
		"--force",
		name,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {
		message := strings.TrimSpace(string(output))

		// A container that was never created or has
		// already been removed requires no cleanup.
		if strings.Contains(
			message,
			"No such container",
		) {
			return nil
		}

		return fmt.Errorf(
			"docker container cleanup failed: %w: %s",
			err,
			message,
		)
	}

	return nil
}

// Shared Docker resource restrictions.
func dockerArguments(
	containerName string,
	image string,
	command ...string,
) []string {

	args := []string{
		"run",
		"--name", containerName,
		"-i",
		"--network=none",
		"--memory=128m",
		"--cpus=0.5",
		"--pids-limit=64",
		"--read-only",
		"--tmpfs=/tmp:rw,noexec,nosuid,size=16m",
		"--user=65534:65534",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		image,
	}

	return append(args, command...)
}

// Internal result shared by both execution modes.
type containerResult struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	TimedOut        bool
	StdoutTruncated bool
	StderrTruncated bool
}

// Executes a Docker container with bounded output,
// resource restrictions and automatic cleanup.
func runContainer(
	code string,
	image string,
	stdoutLimit int,
	command ...string,
) (containerResult, error) {

	containerName, err := newContainerName()
	if err != nil {
		return containerResult{}, err
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		executionTimeout,
	)
	defer cancel()

	args := dockerArguments(
		containerName,
		image,
		command...,
	)

	cmd := exec.CommandContext(ctx, "docker", args...)

	cmd.Stdin = strings.NewReader(code)

	// Bounds additional waiting for subprocess I/O.
	cmd.WaitDelay = 2 * time.Second

	stdout := limitedBuffer{
		maxBytes: stdoutLimit,
	}

	stderr := limitedBuffer{
		maxBytes: maxStderrBytes,
	}

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()

	runErr := cmd.Run()

	duration := time.Since(start)

	// Capture timeout state before cleanup.
	timedOut := ctx.Err() == context.DeadlineExceeded

	result := containerResult{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		Duration:        duration,
		TimedOut:        timedOut,
		ExitCode:        -1,
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}

	if runErr == nil {
		result.ExitCode = 0
	} else {
		var exitErr *exec.ExitError

		if errors.As(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
	}

	// Always attempt cleanup, even after a timeout.
	cleanupErr := removeContainer(containerName)

	if cleanupErr != nil {
		if runErr != nil {
			return result, fmt.Errorf(
				"Docker execution error: %v; cleanup error: %w",
				runErr,
				cleanupErr,
			)
		}

		return result, cleanupErr
	}

	if timedOut {
		result.ExitCode = -1
		return result, nil
	}

	if runErr == nil {
		return result, nil
	}

	var exitErr *exec.ExitError

	if errors.As(runErr, &exitErr) {
		return result, nil
	}

	return result, fmt.Errorf(
		"Docker execution failed: %w",
		runErr,
	)
}

// RunPython preserves the existing raw execution mode.
func RunPython(code string) (Result, error) {

	execution, err := runContainer(
		code,
		pythonImage,
		maxStdoutBytes,
		"python",
		"-I",
		"-",
	)

	result := Result{
		Stdout:          execution.Stdout,
		Stderr:          execution.Stderr,
		ExitCode:        execution.ExitCode,
		Duration:        execution.Duration,
		StdoutTruncated: execution.StdoutTruncated,
		StderrTruncated: execution.StderrTruncated,
	}

	if err != nil {
		return result, err
	}

	if execution.TimedOut {
		message, truncated := appendWithinLimit(
			result.Stderr,
			"\nExecution timed out",
			maxStderrBytes,
		)

		result.Stderr = message

		result.StderrTruncated =
			result.StderrTruncated || truncated

		result.ExitCode = -1
	}

	return result, nil
}

// decodeTraceResponse parses and validates the
// JSON protocol returned by the Python runner.
func decodeTraceResponse(
	output string,
) (*TraceResult, error) {

	if strings.TrimSpace(output) == "" {
		return nil, errors.New(
			"Python tracer returned an empty response",
		)
	}

	var trace TraceResult

	if err := json.Unmarshal(
		[]byte(output),
		&trace,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid Python trace response: %w",
			err,
		)
	}

	switch trace.Status {
	case "completed", "failed", "limit_reached":
		// Supported runner statuses.

	default:
		return nil, fmt.Errorf(
			"unknown trace status: %q",
			trace.Status,
		)
	}

	if trace.TotalSteps < 0 {
		return nil, errors.New(
			"invalid negative trace step count",
		)
	}

	if trace.TotalSteps != len(trace.Events) {
		return nil, fmt.Errorf(
			"trace step count mismatch: expected %d, got %d",
			trace.TotalSteps,
			len(trace.Events),
		)
	}

	if trace.TraceSizeBytes < 0 {
		return nil, errors.New(
			"invalid negative trace size",
		)
	}

	if trace.Status == "failed" && trace.Error == nil {
		return nil, errors.New(
			"failed trace is missing error information",
		)
	}

	if trace.Status == "completed" && trace.Error != nil {
		return nil, errors.New(
			"completed trace contains an execution error",
		)
	}

	if trace.Status == "limit_reached" && trace.Limit == nil {
		return nil, errors.New(
			"trace limit response is missing limit information",
		)
	}

	return &trace, nil
}

// RunPythonTrace executes Python code through
// the containerized tracing engine.
func RunPythonTrace(code string) (TracedResult, error) {

	execution, err := runContainer(
		code,
		tracerImage,
		maxTraceResponseBytes,
	)

	result := TracedResult{
		Duration:        execution.Duration,
		ExitCode:        execution.ExitCode,
		TimedOut:        execution.TimedOut,
		DockerStderr:    execution.Stderr,
		StderrTruncated: execution.StderrTruncated,
	}

	if err != nil {
		return result, err
	}

	// An external timeout may leave incomplete JSON.
	if execution.TimedOut {
		return result, nil
	}

	if execution.ExitCode != 0 {
		return result, fmt.Errorf(
			"Python tracer exited with code %d: %s",
			execution.ExitCode,
			execution.Stderr,
		)
	}

	// Never decode a truncated JSON response.
	if execution.StdoutTruncated {
		return result, fmt.Errorf(
			"trace response exceeded %d bytes",
			maxTraceResponseBytes,
		)
	}

	trace, err := decodeTraceResponse(execution.Stdout)

	if err != nil {
		return result, err
	}

	result.Trace = trace

	return result, nil
}
