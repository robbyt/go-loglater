package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestGroupLogDemo(t *testing.T) {
	// Use buffer instead of os.Stdout for testing
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)

	// Run the group demo function
	logCount, err := GroupLogDemo(handler)
	if err != nil {
		t.Fatalf("GroupLogDemo returned error: %v", err)
	}

	// Verify expected number of logs (5 from service.LogServiceActivity)
	if logCount != 5 {
		t.Errorf("Expected 5 logs, got %d", logCount)
	}

	// Verify log output contains messages from different components
	output := buf.String()

	expectedComponents := []string{
		"component=database",
		"component=http",
	}

	for _, comp := range expectedComponents {
		if !strings.Contains(output, comp) {
			t.Errorf("Expected output to contain '%s', but it doesn't", comp)
		}
	}

	// Verify log groups appear in the output
	expectedGroups := []string{
		"db.",
		"api.",
	}

	for _, group := range expectedGroups {
		if !strings.Contains(output, group) {
			t.Errorf("Expected output to contain '%s' group prefix, but it doesn't", group)
		}
	}

	// Verify expected log messages are present
	expectedMessages := []string{
		"Service started",
		"Connected to database",
		"Query failed",
		"HTTP server listening",
		"Rate limit exceeded",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("Expected output to contain message '%s', but it doesn't", msg)
		}
	}
}
