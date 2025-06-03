package loglater

import (
	"bytes"
	"log/slog"
	"runtime"
	"strings"
	"testing"
)

// TestPCPreservation verifies that program counter information is preserved
// and replayed correctly
func TestPCPreservation(t *testing.T) {
	collector := NewLogCollector(nil)
	logger := slog.New(collector)

	// Get the PC before logging
	var pc uintptr
	callers := make([]uintptr, 1)
	n := runtime.Callers(1, callers)
	if n > 0 {
		pc = callers[0]
	}

	// Log a message (this will have a different PC)
	logger.Info("test message with PC")

	// Get the logs
	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	// Verify PC was captured (should be non-zero)
	if logs[0].PC == 0 {
		t.Error("Expected non-zero PC value")
	}

	// Verify PC is different from our test PC (since log was from different location)
	if logs[0].PC == pc {
		t.Error("PC should be from the logger call site, not test site")
	}

	// Now replay the logs and verify PC is preserved
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		AddSource: true, // Enable source information
	})

	err := collector.PlayLogs(handler)
	if err != nil {
		t.Fatalf("PlayLogs failed: %v", err)
	}

	// Check that source information is present in output
	output := buf.String()
	if !strings.Contains(output, "loglater_pc_test.go") {
		t.Errorf("Expected source file in output, got: %s", output)
	}
}

// TestPCWithGroups verifies PC is preserved when using groups
func TestPCWithGroups(t *testing.T) {
	collector := NewLogCollector(nil)

	// Create logger with group
	logger := slog.New(collector.WithGroup("testgroup"))

	// Log a message
	logger.Info("grouped message")

	// Get the logs
	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	// Verify PC was captured
	if logs[0].PC == 0 {
		t.Error("Expected non-zero PC value with groups")
	}

	// Replay with source information
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		AddSource: true,
	})

	err := collector.PlayLogs(handler)
	if err != nil {
		t.Fatalf("PlayLogs failed: %v", err)
	}

	// Verify source is in output
	output := buf.String()
	if !strings.Contains(output, "source=") {
		t.Errorf("Expected source information in output, got: %s", output)
	}
}

// TestPCWithAttrs verifies PC is preserved when using attributes
func TestPCWithAttrs(t *testing.T) {
	collector := NewLogCollector(nil)

	// Create logger with attributes
	logger := slog.New(collector.WithAttrs([]slog.Attr{
		slog.String("component", "test"),
	}))

	// Log a message
	logger.Info("message with attrs")

	// Get the logs
	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	// Verify PC was captured
	if logs[0].PC == 0 {
		t.Error("Expected non-zero PC value with attrs")
	}
}

// TestCompareOriginalVsReplayed compares original log output with replayed output
func TestCompareOriginalVsReplayed(t *testing.T) {
	// Capture original output
	var originalBuf bytes.Buffer
	originalHandler := slog.NewJSONHandler(&originalBuf, &slog.HandlerOptions{
		AddSource: true,
	})

	// Create collector that forwards to original handler
	collector := NewLogCollector(originalHandler)
	logger := slog.New(collector)

	// Log a message
	logger.Info("test message", "key", "value")

	// Capture replayed output
	var replayedBuf bytes.Buffer
	replayedHandler := slog.NewJSONHandler(&replayedBuf, &slog.HandlerOptions{
		AddSource: true,
	})

	err := collector.PlayLogs(replayedHandler)
	if err != nil {
		t.Fatalf("PlayLogs failed: %v", err)
	}

	// Both should contain source information
	original := originalBuf.String()
	replayed := replayedBuf.String()

	if !strings.Contains(original, "source") {
		t.Error("Original output missing source information")
	}

	if !strings.Contains(replayed, "source") {
		t.Error("Replayed output missing source information")
	}

	// The source file should be the same in both
	if !strings.Contains(original, "loglater_pc_test.go") || !strings.Contains(replayed, "loglater_pc_test.go") {
		t.Error("Source file information doesn't match")
	}
}
