package storage

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRecordStorage(t *testing.T) {
	t.Run("NewRecordStorage", func(t *testing.T) {
		// Create storage with capacity
		storage := NewRecordStorage(10)

		// Verify initial state
		records := storage.GetAll()
		if len(records) != 0 {
			t.Errorf("Expected 0 initial records, got %d", len(records))
		}
	})

	t.Run("AppendAndGetAll", func(t *testing.T) {
		// Create storage
		storage := NewRecordStorage(5)

		// Create test records
		record1 := &Record{
			Time:    time.Now(),
			Level:   slog.LevelInfo,
			Message: "test message 1",
			Attrs:   []slog.Attr{slog.String("key", "value")},
			Groups:  []string{"group1"},
		}

		record2 := &Record{
			Time:    time.Now(),
			Level:   slog.LevelError,
			Message: "test message 2",
			Attrs:   []slog.Attr{slog.Int("count", 42)},
			Groups:  []string{"group1", "group2"},
		}

		// Append records
		storage.Append(record1)
		storage.Append(record2)

		// Get all records
		records := storage.GetAll()

		// Verify length
		if len(records) != 2 {
			t.Fatalf("Expected 2 records, got %d", len(records))
		}

		// Verify record values
		if records[0].Message != "test message 1" || records[0].Level != slog.LevelInfo {
			t.Errorf("Record 1 data not correctly stored")
		}

		if records[1].Message != "test message 2" || records[1].Level != slog.LevelError {
			t.Errorf("Record 2 data not correctly stored")
		}

		// Verify we got a copy, not the original
		records[0].Message = "modified"

		// Get records again
		recordsAgain := storage.GetAll()

		// Check the original wasn't modified
		if recordsAgain[0].Message != "test message 1" {
			t.Errorf("GetAll didn't return a copy: original record was modified")
		}
	})

	t.Run("RecordAttributes", func(t *testing.T) {
		// Create a record with attributes
		record := Record{
			Time:    time.Now(),
			Level:   slog.LevelWarn,
			Message: "test with attrs",
			Attrs: []slog.Attr{
				slog.String("string", "value"),
				slog.Int("int", 42),
				slog.Bool("bool", true),
			},
			Groups: []string{"group1"},
		}

		// Verify attributes
		if len(record.Attrs) != 3 {
			t.Errorf("Expected 3 attributes, got %d", len(record.Attrs))
		}

		// Verify groups
		if len(record.Groups) != 1 || record.Groups[0] != "group1" {
			t.Errorf("Groups not correctly stored: %v", record.Groups)
		}
	})

	t.Run("NewRecord", func(t *testing.T) {
		// Create a slog Record with attributes
		r := slog.NewRecord(time.Now(), slog.LevelError, "test message", 0)
		r.AddAttrs(
			slog.String("key1", "value1"),
			slog.Int("key2", 42),
		)

		// Create groups
		groups := []string{"group1", "group2"}

		// Create a new Record
		record := NewRecord(context.Background(), groups, r)

		// Verify basic fields
		if record.Message != "test message" {
			t.Errorf("Expected message 'test message', got %q", record.Message)
		}

		if record.Level != slog.LevelError {
			t.Errorf("Expected level ERROR, got %v", record.Level)
		}

		// Verify attributes were copied
		if len(record.Attrs) != 2 {
			t.Errorf("Expected 2 attributes, got %d", len(record.Attrs))
		}

		// Verify groups
		if len(record.Groups) != 2 {
			t.Errorf("Expected 2 groups, got %d", len(record.Groups))
		}

		if record.Groups[0] != "group1" || record.Groups[1] != "group2" {
			t.Errorf("Groups not correctly stored: %v", record.Groups)
		}
	})
}
