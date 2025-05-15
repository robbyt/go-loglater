package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/robbyt/go-loglater"
)

func TestGroupLogging(t *testing.T) {
	// Use buffer instead of os.Stdout for testing
	var buf bytes.Buffer

	// Use the demo function to generate the logs
	collector, err := demoGroupLogging(&buf)
	if err != nil {
		t.Fatalf("Failed to run demo: %v", err)
	}

	// Verify number of logs
	logs := collector.GetLogs()
	expectedLogCount := 5
	if len(logs) != expectedLogCount {
		t.Errorf("Expected %d logs, got %d", expectedLogCount, len(logs))
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

	// Test JSON output with a different handler
	var jsonBuf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&jsonBuf, nil)

	// Create a collector with JSON handler
	jsonCollector := loglater.NewLogCollector(nil)
	jsonLogger := slog.New(jsonCollector)

	// Create group loggers and log messages
	jsonLogger.WithGroup("db").Info("Database log", "operation", "query")
	jsonLogger.WithGroup("api").Error("API error", "status", 500)

	// Replay to JSON handler
	playErr := jsonCollector.PlayLogs(jsonHandler)
	if playErr != nil {
		t.Fatalf("PlayLogs failed: %v", playErr)
	}

	// Check JSON output
	jsonOutput := jsonBuf.String()

	// Verify JSON contains proper group structure
	if !strings.Contains(jsonOutput, `"db":`) || !strings.Contains(jsonOutput, `"api":`) {
		t.Errorf("JSON output missing group structure: %s", jsonOutput)
	}
}
