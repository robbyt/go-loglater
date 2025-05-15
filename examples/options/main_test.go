package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/robbyt/go-loglater"
	"github.com/robbyt/go-loglater/storage"
)

func TestSizeLimitedLogs(t *testing.T) {
	// Use the demo function to generate the logs
	collector, err := demoSizeLimitedLogs()
	if err != nil {
		t.Fatalf("Failed to run demo: %v", err)
	}

	// Size filter keeps only the 3 most recent logs
	logs := collector.GetLogs()
	expectedCount := 3
	if len(logs) != expectedCount {
		t.Errorf("Size filter must keep exactly %d logs, found %d", expectedCount, len(logs))
	}

	// Size filter removes the oldest 2 logs
	if len(logs) > 0 && logs[0].Message != "Message 3" {
		t.Errorf("Size filter must remove oldest logs first, expected 'Message 3', found '%s'", logs[0].Message)
	}
}

func TestAgeLimitedLogs(t *testing.T) {
	// Create 1h store for testing
	store := storage.NewRecordStorage(
		storage.WithMaxAge(1*time.Hour),
		storage.WithAsyncCleanup(true),
		storage.WithDebounceTime(100*time.Millisecond), // Short debounce for testing
	)

	// Create a collector with the configured storage
	collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))

	// Create records with specific timestamps
	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)
	recentTime := now.Add(-30 * time.Minute)

	// Create test records with context.Background() and nil groups
	ctx := context.Background()

	// Create old record (2 hours old)
	oldRec := slog.NewRecord(oldTime, slog.LevelInfo, "This is an old message", 0)
	oldRecord := storage.NewRecord(ctx, nil, &oldRec)

	// Create recent record (30 min old)
	recentRec := slog.NewRecord(recentTime, slog.LevelInfo, "This is a recent message", 0)
	recentRecord := storage.NewRecord(ctx, nil, &recentRec)

	// Create current record
	currentRec := slog.NewRecord(now, slog.LevelInfo, "This is a current message", 0)
	currentRecord := storage.NewRecord(ctx, nil, &currentRec)

	// Add records directly to storage for accurate time-based testing
	store.Append(oldRecord)
	store.Append(recentRecord)
	store.Append(currentRecord)

	// Wait a moment for cleanup to run (async)
	time.Sleep(200 * time.Millisecond)

	// Age filter removes old entries, keeps 2 logs (recent and current)
	logs := collector.GetLogs()
	expectedCount := 2
	if len(logs) != expectedCount {
		t.Errorf("Age filter must keep exactly %d logs, found %d", expectedCount, len(logs))
	}

	// Age filter removes old entries (2 hours old)
	for _, log := range logs {
		if log.Message == "This is an old message" {
			t.Errorf("Age filter failed to remove old message from 2 hours ago")
		}
	}
}

func TestCustomCleanup(t *testing.T) {
	// Call the demo function directly
	collector, err := demoCustomCleanup()
	if err != nil {
		t.Fatalf("Failed to run demo: %v", err)
	}

	// Level filter keeps only ERROR level logs
	logs := collector.GetLogs()
	expectedCount := 1
	if len(logs) != expectedCount {
		t.Errorf("Level filter must keep exactly %d ERROR log, found %d", expectedCount, len(logs))
	}

	// Level filter removes non-ERROR logs
	if len(logs) > 0 {
		if logs[0].Level != slog.LevelError {
			t.Errorf("Level filter failed to filter out non-ERROR logs, found log with level %s", logs[0].Level)
		}

		if logs[0].Message != "This is an error message" {
			t.Errorf("Level filter kept wrong message, found '%s' instead of error message", logs[0].Message)
		}
	}
}

func TestErrorFilter(t *testing.T) {
	// Test the error filter directly
	filter := createErrorFilter()

	// Create test records with different levels
	now := time.Now()

	// Create test storage.Record objects
	records := []storage.Record{
		{Time: now, Level: slog.LevelInfo, Message: "info"},
		{Time: now, Level: slog.LevelWarn, Message: "warn"},
		{Time: now, Level: slog.LevelError, Message: "error"},
	}

	// Apply filter
	filtered := filter(records)

	// Error filter keeps only ERROR records
	if len(filtered) != 1 {
		t.Errorf("Error filter function must keep exactly 1 ERROR record, found %d", len(filtered))
	}

	// Error filter removes non-ERROR records
	if len(filtered) > 0 && filtered[0].Level != slog.LevelError {
		t.Errorf("Error filter function failed to filter out non-ERROR records, found %s", filtered[0].Level)
	}
}
