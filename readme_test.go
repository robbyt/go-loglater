package loglater

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/robbyt/go-loglater/storage"
)

// TestBasicUsageExample tests the basic usage example from README lines 25-60
func TestBasicUsageExample(t *testing.T) {
	// Capture stdout to verify immediate output
	var stdoutBuf bytes.Buffer

	// Create a text handler that outputs to our buffer instead of stdout
	textHandler := slog.NewTextHandler(&stdoutBuf, nil)

	// Create collector with the stdout text handler as the base handler
	collector := NewLogCollector(textHandler)

	// Create a logger that uses our collector shim
	logger := slog.New(collector)

	// Log some events (these will be output immediately AND collected)
	logger.Info("Starting application", "version", "1.0.0")
	logger.Warn("Configuration file not found")
	logger.Error("Failed to connect to database", "error", "timeout")

	// Verify immediate output occurred
	immediateOutput := stdoutBuf.String()
	if !strings.Contains(immediateOutput, "Starting application") {
		t.Errorf("Expected immediate output to contain 'Starting application', got: %s", immediateOutput)
	}
	if !strings.Contains(immediateOutput, "Configuration file not found") {
		t.Errorf("Expected immediate output to contain 'Configuration file not found', got: %s", immediateOutput)
	}
	if !strings.Contains(immediateOutput, "Failed to connect to database") {
		t.Errorf("Expected immediate output to contain 'Failed to connect to database', got: %s", immediateOutput)
	}

	// Verify logs were collected
	logs := collector.GetLogs()
	if len(logs) != 3 {
		t.Fatalf("Expected 3 collected logs, got %d", len(logs))
	}

	// Create a new buffer for replay
	var replayBuf bytes.Buffer
	replayHandler := slog.NewTextHandler(&replayBuf, nil)

	// Replay all the collected logs to the same handler
	err := collector.PlayLogs(replayHandler)
	if err != nil {
		t.Fatalf("Error playing logs: %v", err)
	}

	// Verify replay output
	replayOutput := replayBuf.String()
	if !strings.Contains(replayOutput, "Starting application") {
		t.Errorf("Expected replay output to contain 'Starting application', got: %s", replayOutput)
	}
	if !strings.Contains(replayOutput, "version=1.0.0") {
		t.Errorf("Expected replay output to contain 'version=1.0.0', got: %s", replayOutput)
	}
}

// TestDeferredLoggingExample tests the deferred logging example from README lines 64-80
func TestDeferredLoggingExample(t *testing.T) {
	// Create a collector with no output handler
	collector := NewLogCollector(nil)

	// Create a logger that uses our collector
	logger := slog.New(collector)

	// Log some events (these will only be collected, not output)
	logger.Info("This log is just stored, not output")

	// Verify no immediate output occurred (since we have no handler)
	// This is implicit - there's no buffer to check

	// Verify logs were collected
	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 collected log, got %d", len(logs))
	}

	if logs[0].Message != "This log is just stored, not output" {
		t.Errorf("Expected message 'This log is just stored, not output', got: %s", logs[0].Message)
	}

	// Later, play logs to stdout with another handler
	var outputBuf bytes.Buffer
	textHandler := slog.NewTextHandler(&outputBuf, nil)
	err := collector.PlayLogs(textHandler)
	if err != nil {
		t.Fatalf("Error playing logs: %v", err)
	}

	// Verify output only happened during replay
	output := outputBuf.String()
	if !strings.Contains(output, "This log is just stored, not output") {
		t.Errorf("Expected replay output to contain the log message, got: %s", output)
	}
}

// TestWorkingWithGroupsExample tests the groups example from README lines 84-101
func TestWorkingWithGroupsExample(t *testing.T) {
	collector := NewLogCollector(nil)
	logger := slog.New(collector)

	// Create loggers with groups
	dbLogger := logger.WithGroup("db")
	apiLogger := logger.WithGroup("api")

	// Log with different loggers
	dbLogger.Info("Connected to database", "host", "db.example.com")
	apiLogger.Error("API request failed", "endpoint", "/users", "status", 500)

	// Verify logs were collected
	logs := collector.GetLogs()
	if len(logs) != 2 {
		t.Fatalf("Expected 2 collected logs, got %d", len(logs))
	}

	// Play logs to JSON handler - group structure should be preserved
	var jsonBuf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&jsonBuf, nil)
	err := collector.PlayLogs(jsonHandler)
	if err != nil {
		t.Fatalf("Error playing logs: %v", err)
	}

	// Verify JSON output contains group structure
	jsonOutput := jsonBuf.String()
	if !strings.Contains(jsonOutput, `"db"`) {
		t.Errorf("Expected JSON output to contain 'db' group, got: %s", jsonOutput)
	}
	if !strings.Contains(jsonOutput, `"api"`) {
		t.Errorf("Expected JSON output to contain 'api' group, got: %s", jsonOutput)
	}
	if !strings.Contains(jsonOutput, "Connected to database") {
		t.Errorf("Expected JSON output to contain 'Connected to database', got: %s", jsonOutput)
	}
	if !strings.Contains(jsonOutput, "API request failed") {
		t.Errorf("Expected JSON output to contain 'API request failed', got: %s", jsonOutput)
	}
}

// TestCleanupOptionsMaxSize tests the WithMaxSize cleanup option from README
func TestCleanupOptionsMaxSize(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create storage with a maximum size of 2 records
		store := storage.NewRecordStorage(storage.WithMaxSize(2))
		collector := NewLogCollector(nil, WithStorage(store))
		logger := slog.New(collector)

		// Log 4 events (should exceed max size of 2)
		logger.Info("Message 1")
		logger.Info("Message 2")
		logger.Info("Message 3")
		logger.Info("Message 4")

		// Give time for any cleanup to occur
		time.Sleep(10 * time.Millisecond)

		// Should only have the last 2 messages
		logs := collector.GetLogs()
		if len(logs) > 2 {
			t.Errorf("Expected at most 2 logs due to max size cleanup, got %d", len(logs))
		}

		// The remaining logs should be the most recent ones
		if len(logs) >= 1 && !strings.Contains(logs[len(logs)-1].Message, "Message 4") {
			t.Errorf("Expected last message to be 'Message 4', got: %s", logs[len(logs)-1].Message)
		}
	})
}

// TestCleanupOptionsMaxAge tests the WithMaxAge cleanup option from README
func TestCleanupOptionsMaxAge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create storage that keeps only logs from a very short time period
		store := storage.NewRecordStorage(storage.WithMaxAge(1 * time.Millisecond))
		collector := NewLogCollector(nil, WithStorage(store))
		logger := slog.New(collector)

		// Log a message
		logger.Info("Old message")

		// Wait longer than the max age
		time.Sleep(5 * time.Millisecond)

		// Log another message
		logger.Info("New message")

		// Give time for cleanup to occur
		time.Sleep(10 * time.Millisecond)

		// Should only have the new message
		logs := collector.GetLogs()

		// Verify we don't have the old message
		for _, log := range logs {
			if strings.Contains(log.Message, "Old message") {
				t.Errorf("Old message should have been cleaned up but was still present")
			}
		}
	})
}

// TestCleanupOptionsAsyncCleanup tests async cleanup from README
func TestCleanupOptionsAsyncCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create storage with async cleanup enabled and a context to control it
		ctx, cancel := context.WithCancel(context.Background())
		store := storage.NewRecordStorage(
			storage.WithContext(ctx),
			storage.WithMaxSize(2),
			storage.WithAsyncCleanup(true),
			storage.WithDebounceTime(10*time.Millisecond), // Short debounce for testing
		)
		collector := NewLogCollector(nil, WithStorage(store))
		logger := slog.New(collector)

		// Log multiple messages
		logger.Info("Message 1")
		logger.Info("Message 2")
		logger.Info("Message 3")

		// Wait for async cleanup to occur
		time.Sleep(50 * time.Millisecond)

		// Should have cleaned up to max size
		logs := collector.GetLogs()
		if len(logs) > 2 {
			t.Errorf("Expected at most 2 logs after async cleanup, got %d", len(logs))
		}

		// Cancel the context to stop the cleanup worker before test ends
		cancel()
		synctest.Wait() // Wait for cleanup worker to exit
	})
}

// TestCleanupOptionsWithContext tests context cancellation from README
func TestCleanupOptionsWithContext(t *testing.T) {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	store := storage.NewRecordStorage(
		storage.WithContext(ctx),
		storage.WithMaxSize(1000),
		storage.WithAsyncCleanup(true),
	)
	collector := NewLogCollector(nil, WithStorage(store))

	// Cancel the context to stop async cleanup
	cancel()

	// The storage should still work even with cancelled context
	logger := slog.New(collector)
	logger.Info("Test message")

	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log even with cancelled context, got %d", len(logs))
	}
}

// TestCustomCleanupFunction tests the custom cleanup function example from README
func TestCustomCleanupFunction(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Custom cleanup function that keeps only error logs
		customCleanup := func(records []storage.Record) []storage.Record {
			var result []storage.Record
			for _, r := range records {
				if r.Level >= slog.LevelError {
					result = append(result, r)
				}
			}
			return result
		}

		store := storage.NewRecordStorage(storage.WithCleanupFunc(customCleanup))
		collector := NewLogCollector(nil, WithStorage(store))
		logger := slog.New(collector)

		// Log messages at different levels
		logger.Info("Info message")
		logger.Warn("Warning message")
		logger.Error("Error message")

		// Trigger cleanup by logging another message
		logger.Debug("Debug message")

		// Give time for cleanup
		time.Sleep(10 * time.Millisecond)

		// Should only have error logs
		logs := collector.GetLogs()
		for _, log := range logs {
			if log.Level < slog.LevelError {
				t.Errorf("Expected only error level logs, but found level %v with message: %s", log.Level, log.Message)
			}
		}

		// Should have the error message
		found := false
		for _, log := range logs {
			if strings.Contains(log.Message, "Error message") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find error message in logs after cleanup")
		}
	})
}

// TestREADMEExampleFormat tests that the actual format matches what's shown in README
func TestREADMEExampleFormat(t *testing.T) {
	// This test ensures the exact code from README works without modification

	// Simulate the main function from the README
	var buf bytes.Buffer

	// Create a text handler that outputs to our buffer
	textHandler := slog.NewTextHandler(&buf, nil)

	// Create collector with the stdout text handler as the base handler
	collector := NewLogCollector(textHandler)

	// Create a logger that uses our collector shim
	logger := slog.New(collector)

	// Log some events (these will be output immediately AND collected)
	logger.Info("Starting application", "version", "1.0.0")
	logger.Warn("Configuration file not found")
	logger.Error("Failed to connect to database", "error", "timeout")

	// Clear buffer for replay test
	buf.Reset()

	// Replay all the collected logs to the same handler
	err := collector.PlayLogs(textHandler)
	if err != nil {
		t.Fatalf("Error playing logs: %v", err)
	}

	// Verify the output format is reasonable
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of output, got %d: %v", len(lines), lines)
	}
}
