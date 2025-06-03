package storage

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func BenchmarkRecordStorage_Append(b *testing.B) {
	b.Run("Synchronous", func(b *testing.B) {
		cases := []struct {
			name string
			size int
			adds int
		}{
			{"0", 0, 10},
			{"1", 1, 10},
			{"10", 10, 10},
			{"20", 20, 10},
		}

		for _, tc := range cases {
			b.Run(tc.name, func(b *testing.B) {
				store := NewRecordStorage(WithPreallocation(tc.size))
				records := make([]*Record, tc.adds)
				for i := range tc.adds {
					records[i] = &Record{
						Time:    time.Now(),
						Level:   0,
						Message: "test",
						Attrs:   nil,
					}
				}

				b.ResetTimer()
				for b.Loop() {
					for _, rec := range records {
						store.Append(rec)
					}
				}
			})
		}
	})

	b.Run("Asynchronous", func(b *testing.B) {
		cases := []struct {
			name string
			size int
			adds int
		}{
			{"0", 0, 10},
			{"1", 1, 10},
			{"10", 10, 10},
			{"20", 20, 10},
		}

		for _, tc := range cases {
			b.Run(tc.name, func(b *testing.B) {
				store := NewRecordStorage(WithPreallocation(tc.size))
				rec := &Record{
					Time:    time.Now(),
					Level:   0,
					Message: "test",
					Attrs:   nil,
				}
				b.ResetTimer()
				for b.Loop() {
					var wg sync.WaitGroup
					wg.Add(tc.adds)
					for range tc.adds {
						go func() {
							store.Append(rec)
							wg.Done()
						}()
					}
					wg.Wait()
				}
			})
		}
	})
}

func BenchmarkRecordStorage_GetAll(b *testing.B) {
	b.Run("Synchronous", func(b *testing.B) {
		cases := []struct {
			name string
			size int
			gets int
		}{
			{"0", 0, 1},
			{"1", 1, 1},
			{"10", 10, 1},
			{"20", 20, 1},
		}

		rec := &Record{
			Time:    time.Now(),
			Level:   0,
			Message: "test",
			Attrs:   nil,
		}

		for _, tc := range cases {
			b.Run(tc.name, func(b *testing.B) {
				store := NewRecordStorage(WithPreallocation(tc.size))
				// Setup: fill the store with some data
				for i := 0; i < tc.size; i++ {
					store.Append(rec)
				}
				b.ResetTimer()
				for b.Loop() {
					for i := 0; i < tc.gets; i++ {
						_ = store.GetAll()
					}
				}
			})
		}
	})

	b.Run("Asynchronous", func(b *testing.B) {
		cases := []struct {
			name string
			size int
			gets int
		}{
			{"0", 0, 10},
			{"1", 1, 10},
			{"10", 10, 10},
			{"20", 20, 10},
		}

		rec := &Record{
			Time:    time.Now(),
			Level:   0,
			Message: "test",
			Attrs:   nil,
		}

		for _, tc := range cases {
			b.Run(tc.name, func(b *testing.B) {
				store := NewRecordStorage(WithPreallocation(tc.size))
				// Setup: fill the store with some data
				for i := 0; i < tc.size; i++ {
					store.Append(rec)
				}
				b.ResetTimer()
				for b.Loop() {
					var wg sync.WaitGroup
					wg.Add(tc.gets)
					for i := 0; i < tc.gets; i++ {
						go func() {
							_ = store.GetAll()
							wg.Done()
						}()
					}
					wg.Wait()
				}
			})
		}
	})
}

func TestRecordStorage(t *testing.T) {
	t.Run("NewRecordStorage", func(t *testing.T) {
		// Create storage with capacity
		storage := NewRecordStorage()

		// Verify initial state
		records := storage.GetAll()
		if len(records) != 0 {
			t.Errorf("Expected 0 initial records, got %d", len(records))
		}
	})

	t.Run("AppendAndGetAll", func(t *testing.T) {
		// Create storage
		storage := NewRecordStorage(WithPreallocation(5))

		// Create test records
		record1 := &Record{
			Time:    time.Now(),
			Level:   slog.LevelInfo,
			Message: "test message 1",
			Attrs:   []slog.Attr{slog.String("key", "value")},
		}

		record2 := &Record{
			Time:    time.Now(),
			Level:   slog.LevelError,
			Message: "test message 2",
			Attrs:   []slog.Attr{slog.Int("count", 42)},
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
		}

		// Verify attributes
		if len(record.Attrs) != 3 {
			t.Errorf("Expected 3 attributes, got %d", len(record.Attrs))
		}

	})

	t.Run("NewRecord", func(t *testing.T) {
		// Create a slog Record with attributes
		r := slog.NewRecord(time.Now(), slog.LevelError, "test message", 0)
		r.AddAttrs(
			slog.String("key1", "value1"),
			slog.Int("key2", 42),
		)

		// Create a new Record
		record := NewRecord(context.Background(), nil, &r)

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

	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		storage := NewRecordStorage()
		const numGoroutines = 10
		const recordsPerGoroutine = 100

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < recordsPerGoroutine; j++ {
					record := &Record{
						Time:    time.Now(),
						Level:   slog.LevelInfo,
						Message: "concurrent test",
						Attrs:   []slog.Attr{slog.Int("goroutine", id), slog.Int("index", j)},
					}
					storage.Append(record)
				}
			}(i)
		}

		wg.Wait()

		records := storage.GetAll()
		expectedCount := numGoroutines * recordsPerGoroutine
		if len(records) != expectedCount {
			t.Errorf("Expected %d records, got %d", expectedCount, len(records))
		}
	})

	t.Run("EmptyStorage", func(t *testing.T) {
		storage := NewRecordStorage()
		records := storage.GetAll()

		if records == nil {
			t.Error("GetAll should return empty slice, not nil")
		}

		if len(records) != 0 {
			t.Errorf("Expected empty slice, got %d records", len(records))
		}
	})

	t.Run("SingleRecord", func(t *testing.T) {
		storage := NewRecordStorage()
		record := &Record{
			Time:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Level:   slog.LevelDebug,
			Message: "single record test",
			PC:      12345,
			Attrs:   []slog.Attr{slog.String("test", "value")},
		}

		storage.Append(record)
		records := storage.GetAll()

		if len(records) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(records))
		}

		retrieved := records[0]
		if retrieved.Message != "single record test" {
			t.Errorf("Message mismatch: got %q", retrieved.Message)
		}

		if retrieved.Level != slog.LevelDebug {
			t.Errorf("Level mismatch: got %v", retrieved.Level)
		}

		if retrieved.PC != 12345 {
			t.Errorf("PC mismatch: got %d", retrieved.PC)
		}

		if len(retrieved.Attrs) != 1 {
			t.Errorf("Attrs length mismatch: got %d", len(retrieved.Attrs))
		}
	})
}

func TestMemStorageCleanup(t *testing.T) {
	t.Run("CleanupFunctionality", func(t *testing.T) {
		cleanupCalled := false
		cleanupFunc := func(records []Record) []Record {
			cleanupCalled = true
			if len(records) > 2 {
				return records[len(records)-2:]
			}
			return records
		}

		storage := NewRecordStorage(WithCleanupFunc(cleanupFunc))

		record1 := &Record{Message: "record 1"}
		record2 := &Record{Message: "record 2"}
		record3 := &Record{Message: "record 3"}

		storage.Append(record1)
		storage.Append(record2)
		storage.Append(record3)

		storage.performCleanup()

		if !cleanupCalled {
			t.Error("Cleanup function was not called")
		}

		records := storage.GetAll()
		if len(records) != 2 {
			t.Errorf("Expected 2 records after cleanup, got %d", len(records))
		}
	})

	t.Run("AsyncCleanupDisabled", func(t *testing.T) {
		storage := NewRecordStorage(WithCleanupFunc(func(records []Record) []Record {
			return records[:0]
		}))

		if storage.asyncCleanupEnabled {
			t.Error("Async cleanup should be disabled by default")
		}

		record := &Record{Message: "test"}
		storage.Append(record)

		records := storage.GetAll()
		if len(records) != 0 {
			t.Errorf("Expected 0 records after sync cleanup, got %d", len(records))
		}
	})

	t.Run("NoCleanupFunction", func(t *testing.T) {
		storage := NewRecordStorage()

		record := &Record{Message: "test"}
		storage.Append(record)

		storage.performCleanup()

		records := storage.GetAll()
		if len(records) != 1 {
			t.Errorf("Expected 1 record when no cleanup func, got %d", len(records))
		}
	})
}
