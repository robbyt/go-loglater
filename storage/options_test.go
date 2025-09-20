package storage

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"
)

func createTestRecord(ctx context.Context, time time.Time, level slog.Level, msg string) *Record {
	rec := slog.NewRecord(time, level, msg, 0)
	return NewRecord(ctx, nil, &rec)
}

func TestWithMaxSize(t *testing.T) {
	// Create storage with max size of 2
	store := NewRecordStorage(WithMaxSize(2))

	// Add 3 records
	store.Append(createTestRecord(t.Context(), time.Now().Add(-2*time.Hour), slog.LevelInfo, "Message 1"))
	store.Append(createTestRecord(t.Context(), time.Now().Add(-1*time.Hour), slog.LevelInfo, "Message 2"))
	store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Message 3"))

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
	store.Append(createTestRecord(t.Context(), time.Now().Add(-2*time.Hour), slog.LevelInfo, "Message 1"))
	store.Append(createTestRecord(t.Context(), time.Now().Add(-1*time.Hour), slog.LevelInfo, "Message 2"))
	store.Append(createTestRecord(t.Context(), time.Now().Add(-30*time.Minute), slog.LevelInfo, "Message 3"))

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
	store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Info message"))
	store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelWarn, "Warning message"))
	store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelError, "Error message"))

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
	synctest.Test(t, func(t *testing.T) {
		// Create storage with multiple options
		store := NewRecordStorage(
			WithMaxSize(3),
			WithAsyncCleanup(true),
			WithContext(t.Context()),
			WithDebounceTime(100*time.Millisecond),
		)

		// Add 5 records - should be trimmed to 3 by MaxSize after async cleanup
		store.Append(createTestRecord(t.Context(), time.Now().Add(-60*time.Minute), slog.LevelInfo, "Message 1"))
		store.Append(createTestRecord(t.Context(), time.Now().Add(-45*time.Minute), slog.LevelInfo, "Message 2"))
		store.Append(createTestRecord(t.Context(), time.Now().Add(-15*time.Minute), slog.LevelInfo, "Message 3"))
		store.Append(createTestRecord(t.Context(), time.Now().Add(-10*time.Minute), slog.LevelInfo, "Message 4"))
		store.Append(createTestRecord(t.Context(), time.Now().Add(-5*time.Minute), slog.LevelInfo, "Message 5"))

		// Sleep to advance time past the debounce period (100ms + buffer)
		// In synctest, time.Sleep blocks the test goroutine, allowing the fake clock
		// to advance by the sleep duration, which triggers the debounce timer
		time.Sleep(200 * time.Millisecond)

		// Wait for async cleanup to complete
		synctest.Wait()

		// Get all logs - should only have the 3 most recent messages
		logs := store.GetAll()
		if len(logs) != 3 {
			t.Errorf("Expected 3 records, got %d", len(logs))
		}

		// Check that the most recent messages remain (Message 3, 4, 5)
		if len(logs) > 0 && logs[0].Message != "Message 3" {
			t.Errorf("Expected first message to be 'Message 3', got '%s'", logs[0].Message)
		}
		if len(logs) > 2 && logs[2].Message != "Message 5" {
			t.Errorf("Expected last message to be 'Message 5', got '%s'", logs[2].Message)
		}
	})
}

func TestWithContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create a cancellable context
		ctx, cancel := context.WithCancel(t.Context())

		// Create storage with context
		store := NewRecordStorage(
			WithContext(ctx),
			WithAsyncCleanup(true),
			WithMaxSize(5),
		)

		// Add some records
		store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Message 1"))
		store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Message 2"))

		// Wait for any initial cleanup to settle
		synctest.Wait()

		// Records should still be there since max size is 5
		logs := store.GetAll()
		if len(logs) != 2 {
			t.Errorf("Expected 2 records, got %d", len(logs))
		}

		// Cancel the context to stop the async worker
		cancel()

		// Wait for cancellation to take effect
		synctest.Wait()

		// Add more records to exceed max size
		for range 5 {
			store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "New Message"))
		}

		// Wait for any cleanup to complete (should not happen since worker is stopped)
		synctest.Wait()

		// Should now have 7 records (previous 2 + 5 new ones) since cleanup worker should be stopped
		logs = store.GetAll()
		if len(logs) != 7 {
			t.Errorf("Expected 7 records after context cancellation, got %d", len(logs))
		}
	})
}

func TestWithDebounceTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Create storage with a debounce time to test the timer functionality
		store := NewRecordStorage(
			WithDebounceTime(1*time.Second),
			WithAsyncCleanup(true),
			WithContext(t.Context()),
			WithMaxSize(2),
		)

		// Add 3 records quickly
		store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Message 1"))
		store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Message 2"))
		store.Append(createTestRecord(t.Context(), time.Now(), slog.LevelInfo, "Message 3"))

		// Check immediately - should still have all 3 records since cleanup is debounced
		logs := store.GetAll()
		if len(logs) != 3 {
			t.Errorf("Expected 3 records before debounce time, got %d", len(logs))
		}

		// Sleep to advance time past the debounce period (1s + buffer)
		// In synctest, time.Sleep blocks the test goroutine, allowing the fake clock
		// to advance by the sleep duration, which triggers the debounce timer
		time.Sleep(1200 * time.Millisecond)

		// Wait for async cleanup to complete
		synctest.Wait()

		// Now check again - should have only 2 records after debounce time passes
		logs = store.GetAll()
		if len(logs) != 2 {
			t.Errorf("Expected 2 records after debounce time, got %d", len(logs))
		}

		// Verify the most recent records were kept
		if len(logs) > 0 && logs[len(logs)-1].Message != "Message 3" {
			t.Errorf("Expected last message to be 'Message 3', got '%s'", logs[len(logs)-1].Message)
		}
	})
}
