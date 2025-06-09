package storage

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func createTestRecord(time time.Time, level slog.Level, msg string) *Record {
	rec := slog.NewRecord(time, level, msg, 0)
	return NewRecord(context.Background(), nil, &rec)
}

func TestWithMaxSize(t *testing.T) {
	// Create storage with max size of 2
	store := NewRecordStorage(WithMaxSize(2))

	// Add 3 records
	store.Append(createTestRecord(time.Now().Add(-2*time.Hour), slog.LevelInfo, "Message 1"))
	store.Append(createTestRecord(time.Now().Add(-1*time.Hour), slog.LevelInfo, "Message 2"))
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Message 3"))

	// Get all logs - should only have the 2 most recent
	logs := store.GetAll()
	if len(logs) != 2 {
		t.Errorf("Expected 2 records, got %d", len(logs))
	}

	// First record should be the second message since the first one was trimmed
	if len(logs) > 0 && logs[0].Message != "Message 2" {
		t.Errorf("Expected first log to be 'Message 2', got '%s'", logs[0].Message)
	}
}

func TestWithMaxAge(t *testing.T) {
	// Create storage with max age of 90 minutes
	store := NewRecordStorage(WithMaxAge(90 * time.Minute))

	// Add 3 records with different ages
	store.Append(createTestRecord(time.Now().Add(-2*time.Hour), slog.LevelInfo, "Message 1"))
	store.Append(createTestRecord(time.Now().Add(-1*time.Hour), slog.LevelInfo, "Message 2"))
	store.Append(createTestRecord(time.Now().Add(-30*time.Minute), slog.LevelInfo, "Message 3"))

	// Get all logs - should only have the newer ones
	logs := store.GetAll()
	if len(logs) != 2 {
		t.Errorf("Expected 2 records, got %d", len(logs))
	}

	// Should contain Message 2 and Message 3
	if len(logs) > 0 && logs[0].Message != "Message 2" {
		t.Errorf("Expected first log to be 'Message 2', got '%s'", logs[0].Message)
	}
}

func TestWithCleanupFunc(t *testing.T) {
	// Create a custom cleanup function that keeps only warnings or higher
	levelFilter := func(records []Record) []Record {
		var result []Record
		for _, r := range records {
			if r.Level >= slog.LevelWarn {
				result = append(result, r)
			}
		}
		return result
	}

	// Create storage with custom cleanup
	store := NewRecordStorage(WithCleanupFunc(levelFilter))

	// Add mixed level records
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Info message"))
	store.Append(createTestRecord(time.Now(), slog.LevelWarn, "Warning message"))
	store.Append(createTestRecord(time.Now(), slog.LevelError, "Error message"))

	// Get all logs - should only have warnings and errors
	logs := store.GetAll()
	if len(logs) != 2 {
		t.Errorf("Expected 2 records, got %d", len(logs))
	}

	// Check that only warning and error messages remain
	for _, log := range logs {
		if log.Level < slog.LevelWarn {
			t.Errorf("Expected only warnings and errors, but found: %s", log.Message)
		}
	}
}

func TestCombiningOptions(t *testing.T) {
	// Create storage with multiple options
	store := NewRecordStorage(
		WithMaxSize(5),
		WithMaxAge(30*time.Minute),
		WithAsyncCleanup(true),
		WithDebounceTime(100*time.Millisecond), // Short debounce for testing
	)

	// Add a mix of records
	store.Append(createTestRecord(time.Now().Add(-60*time.Minute), slog.LevelInfo, "Old Message 1"))
	store.Append(createTestRecord(time.Now().Add(-45*time.Minute), slog.LevelInfo, "Old Message 2"))
	store.Append(createTestRecord(time.Now().Add(-15*time.Minute), slog.LevelInfo, "Recent Message 1"))
	store.Append(createTestRecord(time.Now().Add(-10*time.Minute), slog.LevelInfo, "Recent Message 2"))
	store.Append(createTestRecord(time.Now().Add(-5*time.Minute), slog.LevelInfo, "Recent Message 3"))

	// Since cleanup is async, we need to wait a bit
	time.Sleep(200 * time.Millisecond)

	// Get all logs - should only have recent messages
	logs := store.GetAll()
	if len(logs) != 3 {
		t.Errorf("Expected 3 records, got %d", len(logs))
	}

	// Check that only recent messages remain
	for _, log := range logs {
		if log.Time.Before(time.Now().Add(-30 * time.Minute)) {
			t.Errorf("Expected only recent messages, but found: %s", log.Message)
		}
	}
}

func TestWithContext(t *testing.T) {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create storage with context
	store := NewRecordStorage(
		WithContext(ctx),
		WithAsyncCleanup(true),
		WithMaxSize(5),
	)

	// Add some records
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Message 1"))
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Message 2"))

	// Trigger cleanup
	time.Sleep(200 * time.Millisecond)

	// Records should still be there since max size is 5
	logs := store.GetAll()
	if len(logs) != 2 {
		t.Errorf("Expected 2 records, got %d", len(logs))
	}

	// Cancel the context to stop the async worker
	cancel()

	// Wait for cancel to take effect
	time.Sleep(200 * time.Millisecond)

	// Add more records to exceed max size
	for range 5 {
		store.Append(createTestRecord(time.Now(), slog.LevelInfo, "New Message"))
	}

	// Wait for any cleanup to happen
	time.Sleep(200 * time.Millisecond)

	// Should now have 7 records (previous 2 + 5 new ones) since cleanup worker should be stopped
	logs = store.GetAll()
	if len(logs) != 7 {
		t.Errorf("Expected 7 records after context cancellation, got %d", len(logs))
	}
}

func TestWithDebounceTime(t *testing.T) {
	// Create storage with a very long debounce time (1 second)
	store := NewRecordStorage(
		WithDebounceTime(1*time.Second),
		WithAsyncCleanup(true),
		WithMaxSize(2),
	)

	// Add 3 records quickly
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Message 1"))
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Message 2"))
	store.Append(createTestRecord(time.Now(), slog.LevelInfo, "Message 3"))

	// Check immediately - should still have all 3 records since cleanup is debounced
	logs := store.GetAll()
	if len(logs) != 3 {
		t.Errorf("Expected 3 records before debounce time, got %d", len(logs))
	}

	// Wait for the debounce time to pass
	time.Sleep(1200 * time.Millisecond)

	// Now check again - should have only 2 records after debounce time passes
	logs = store.GetAll()
	if len(logs) != 2 {
		t.Errorf("Expected 2 records after debounce time, got %d", len(logs))
	}

	// Verify the most recent records were kept
	if len(logs) > 0 && logs[len(logs)-1].Message != "Message 3" {
		t.Errorf("Expected last message to be 'Message 3', got '%s'", logs[len(logs)-1].Message)
	}
}
