package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogDemo(t *testing.T) {
	// Use buffer instead of os.Stdout for testing
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)

	// Run the demo function
	logCount, err := LogDemo(handler)
	if err != nil {
		t.Fatalf("LogDemo returned error: %v", err)
	}

	// Verify expected number of logs
	if logCount != 3 {
		t.Errorf("Expected 3 logs, got %d", logCount)
	}

	// Verify log output contains our messages
	output := buf.String()

	expectedMessages := []string{
		"Starting demo",
		"Just a warning message",
		"This is an error message",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("Expected output to contain '%s', but it doesn't", msg)
		}
	}

	// Verify expected log levels appear in output
	expectedLevels := []string{
		"level=INFO",
		"level=WARN",
		"level=ERROR",
	}

	for _, level := range expectedLevels {
		if !strings.Contains(output, level) {
			t.Errorf("Expected output to contain '%s', but it doesn't", level)
		}
	}
}
