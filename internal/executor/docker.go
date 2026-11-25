package executor

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	executionTimeout = 5 * time.Second
	cleanupTimeout   = 5 * time.Second
	maxStdoutBytes   = 1 * 1024 * 1024 // 1 MB
	maxStderrBytes   = 1 * 1024 * 1024 // 1 MB
)

type Result struct {
	Stdout          string
	Stderr          string
	ExitCode        int
	Duration        time.Duration
	StdoutTruncated bool
	StderrTruncated bool
}

// limitedBuffer stores at most maxBytes while accepting all writes.
// Extra output is discarded instead of being stored in memory.
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

// appendWithinLimit appends a message without exceeding the size limit.
func appendWithinLimit(current, message string, maxBytes int) (string, bool) {
	if len(message) >= maxBytes {
		return message[:maxBytes], true
	}

	if len(current)+len(message) <= maxBytes {
		return current + message, false
	}

	keepBytes := maxBytes - len(message)
	return current[:keepBytes] + message, true
}

// newContainerName generates a unique name for each execution container.
func newContainerName() (string, error) {
	randomBytes := make([]byte, 16)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("could not generate container name: %w", err)
	}

	return "algotrace-" + hex.EncodeToString(randomBytes), nil
}

// removeContainer forcibly removes the execution container.
// It uses its own timeout because the execution context may already have expired.
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
		return fmt.Errorf(
			"docker container cleanup failed: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

func RunPython(code string) (Result, error) {
	containerName, err := newContainerName()
	if err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		executionTimeout,
	)
	defer cancel()

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
		"python:3.12-alpine",
		"python",
		"-I",
		"-",
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(code)

	var stdout limitedBuffer
	stdout.maxBytes = maxStdoutBytes

	var stderr limitedBuffer
	stderr.maxBytes = maxStderrBytes

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	result := Result{
		Stdout:          stdout.String(),
		Stderr:          stderr.String(),
		Duration:        duration,
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}

	// Always attempt container cleanup, including after timeouts.
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

	// Handle execution timeout after cleanup.
	if ctx.Err() == context.DeadlineExceeded {
		result.Stderr, result.StderrTruncated = appendWithinLimit(
			result.Stderr,
			"\nExecution timed out",
			maxStderrBytes,
		)
		result.ExitCode = -1
		return result, nil
	}

	if runErr == nil {
		result.ExitCode = 0
		return result, nil
	}

	if exitErr, ok := runErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, fmt.Errorf(
		"could not start Docker execution: %w",
		runErr,
	)
}
