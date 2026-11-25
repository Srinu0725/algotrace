package executor

import (
	"strings"
	"testing"
)

func TestLimitedBufferWithinLimit(t *testing.T) {
	var b limitedBuffer
	b.maxBytes = 100

	input := "Hello AlgoTrace"

	n, err := b.Write([]byte(input))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != len(input) {
		t.Fatalf("expected %d bytes written, got %d", len(input), n)
	}

	if b.String() != input {
		t.Fatalf("expected %q, got %q", input, b.String())
	}

	if b.truncated {
		t.Fatal("output should not be marked as truncated")
	}
}

func TestLimitedBufferTruncatesExcessOutput(t *testing.T) {
	var b limitedBuffer
	b.maxBytes = 10

	input := "0123456789ABCDEFGHIJ"

	n, err := b.Write([]byte(input))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != len(input) {
		t.Fatalf("expected all %d input bytes to be accepted, got %d", len(input), n)
	}

	if b.String() != "0123456789" {
		t.Fatalf("unexpected stored output: %q", b.String())
	}

	if !b.truncated {
		t.Fatal("expected output to be marked as truncated")
	}
}

func TestLimitedBufferMultipleWrites(t *testing.T) {
	var b limitedBuffer
	b.maxBytes = 10

	_, err := b.Write([]byte("12345"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = b.Write([]byte("67890"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = b.Write([]byte("EXTRA"))
	if err != nil {
		t.Fatal(err)
	}

	if b.String() != "1234567890" {
		t.Fatalf("unexpected output: %q", b.String())
	}

	if !b.truncated {
		t.Fatal("expected output to be truncated")
	}
}

func TestLimitedBufferZeroLimit(t *testing.T) {
	var b limitedBuffer
	b.maxBytes = 0

	n, err := b.Write([]byte("Hello"))

	if err != nil {
		t.Fatal(err)
	}

	if n != 5 {
		t.Fatalf("expected 5 bytes accepted, got %d", n)
	}

	if b.String() != "" {
		t.Fatalf("expected empty output, got %q", b.String())
	}

	if !b.truncated {
		t.Fatal("expected output to be marked as truncated")
	}
}

func TestAppendWithinLimit(t *testing.T) {
	current := "Hello"
	message := " World"

	result, truncated := appendWithinLimit(current, message, 20)

	if result != "Hello World" {
		t.Fatalf("unexpected result: %q", result)
	}

	if truncated {
		t.Fatal("output should not be truncated")
	}
}

func TestAppendWithinLimitTruncates(t *testing.T) {
	current := strings.Repeat("A", 10)
	message := " TIMEOUT"

	result, truncated := appendWithinLimit(current, message, 12)

	if len(result) != 12 {
		t.Fatalf("expected 12 bytes, got %d", len(result))
	}

	if !strings.HasSuffix(result, message) {
		t.Fatalf("expected timeout message at the end, got %q", result)
	}

	if !truncated {
		t.Fatal("expected truncation to be reported")
	}
}
