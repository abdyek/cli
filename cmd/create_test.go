package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPromptRequiredLineRejectsEmptyInput(t *testing.T) {
	inputs := []string{"", "   ", "admin"}
	calls := 0

	output := captureStdout(t, func() {
		got := promptRequiredLine("Username: ", "   ⚠️  Username cannot be empty", func() string {
			value := inputs[calls]
			calls++
			return value
		})

		if got != "admin" {
			t.Fatalf("promptRequiredLine() = %q, want %q", got, "admin")
		}
	})

	if calls != len(inputs) {
		t.Fatalf("readLine called %d times, want %d", calls, len(inputs))
	}

	if warnCount := strings.Count(output, "Username cannot be empty"); warnCount != 2 {
		t.Fatalf("warning printed %d times, want 2\noutput: %q", warnCount, output)
	}
}

func TestPromptRequiredLineAcceptsFirstNonEmptyInput(t *testing.T) {
	calls := 0

	output := captureStdout(t, func() {
		got := promptRequiredLine("Password: ", "   ⚠️  Password cannot be empty", func() string {
			calls++
			return "secret"
		})

		if got != "secret" {
			t.Fatalf("promptRequiredLine() = %q, want %q", got, "secret")
		}
	})

	if calls != 1 {
		t.Fatalf("readLine called %d times, want 1", calls)
	}

	if strings.Contains(output, "cannot be empty") {
		t.Fatalf("unexpected warning in output: %q", output)
	}
}

func TestPromptRequiredLineTrimsWhitespace(t *testing.T) {
	got := promptRequiredLine("Username: ", "   ⚠️  Username cannot be empty", func() string {
		return "  admin  "
	})

	if got != "admin" {
		t.Fatalf("promptRequiredLine() = %q, want %q", got, "admin")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	previousStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = previousStdout
	})

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	os.Stdout = previousStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}

	return buf.String()
}
