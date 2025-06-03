package loglater

import (
	"log/slog"
	"sync"
	"testing"
)

// TestConcurrentLoggingWithSharedStorage tests for race conditions when multiple
// collectors created via WithAttrs/WithGroup share the same storage
func TestConcurrentLoggingWithSharedStorage(t *testing.T) {
	t.Parallel()
	// Create base collector
	collector := NewLogCollector(nil)

	// Create derived collectors that share the same storage
	collector1 := collector.WithAttrs([]slog.Attr{
		slog.String("collector", "1"),
	})
	collector2 := collector.WithAttrs([]slog.Attr{
		slog.String("collector", "2"),
	})
	collector3 := collector.WithGroup("group1")

	// Create loggers from the collectors
	logger1 := slog.New(collector1)
	logger2 := slog.New(collector2)
	logger3 := slog.New(collector3)

	// Concurrent logging
	var wg sync.WaitGroup
	iterations := 1000

	wg.Add(3)

	// Logger 1 writes
	go func() {
		defer wg.Done()
		for i := range iterations {
			logger1.Info("test message", "index", i, "logger", 1)
		}
	}()

	// Logger 2 writes
	go func() {
		defer wg.Done()
		for i := range iterations {
			logger2.Info("test message", "index", i, "logger", 2)
		}
	}()

	// Logger 3 writes
	go func() {
		defer wg.Done()
		for i := range iterations {
			logger3.Info("test message", "index", i, "logger", 3)
		}
	}()

	wg.Wait()

	// Verify we have all the logs
	logs := collector.GetLogs()
	expectedCount := iterations * 3
	if len(logs) != expectedCount {
		t.Errorf("Expected %d logs, got %d", expectedCount, len(logs))
	}
}

// TestAttributeModificationRace tests if modifying attributes after record creation
// causes race conditions
func TestAttributeModificationRace(t *testing.T) {
	t.Parallel()
	collector := NewLogCollector(nil)
	logger := slog.New(collector)

	var wg sync.WaitGroup
	wg.Add(2)

	// One goroutine creates loggers with attributes
	go func() {
		defer wg.Done()
		for i := range 100 {
			subLogger := logger.WithGroup("group").With("iteration", i)
			subLogger.Info("message from goroutine 1")
		}
	}()

	// Another goroutine reads logs concurrently
	go func() {
		defer wg.Done()
		for range 100 {
			logs := collector.GetLogs()
			// Just accessing the logs to trigger potential races
			_ = logs
		}
	}()

	wg.Wait()
}

// TestConcurrentPlayLogs tests for race conditions during log replay
func TestConcurrentPlayLogs(t *testing.T) {
	t.Parallel()
	collector := NewLogCollector(nil)
	logger := slog.New(collector)

	// Generate some logs
	for i := range 100 {
		logger.Info("test message", "index", i)
	}

	// setup concurrent replay
	var wg sync.WaitGroup
	wg.Add(3)

	for range 3 {
		go func() {
			defer wg.Done()
			handler := slog.NewTextHandler(&discardWriter{}, nil)
			err := collector.PlayLogs(handler)
			if err != nil {
				t.Errorf("PlayLogs failed: %v", err)
			}
		}()
	}

	wg.Wait()
}

// discardWriter discards all writes
type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
