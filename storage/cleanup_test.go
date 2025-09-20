package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// setupTestStorage creates a storage with the specified number of test records
func setupTestStorage(tb testing.TB, size int, opts ...Option) (*MemStorage, []*Record) {
	tb.Helper()
	storage := NewRecordStorage(opts...)
	records := make([]*Record, size)

	// Create test records with varying timestamps
	// This is important for maxAgeCleanup tests
	baseTime := time.Now().Add(-time.Hour) // Base time 1 hour ago

	for i := range size {
		records[i] = &Record{
			Time:    baseTime.Add(time.Duration(i) * time.Second), // Each record is 1 second newer
			Level:   slog.LevelInfo,
			Message: "test message",
			Attrs:   []slog.Attr{slog.Int("index", i)},
		}
		storage.Append(records[i])
	}

	return storage, records
}

// TestCleanupFunctions ensures our cleanup functions work correctly
// This helps maintain coverage alongside the benchmarks
func TestCleanupFunctions(t *testing.T) {
	t.Run("MaxSizeCleanup", func(t *testing.T) {
		// Test with empty slice
		records := []Record{}
		cleanupFn := maxSizeCleanup(10)
		result := cleanupFn(records)
		if len(result) != 0 {
			t.Errorf("Expected empty result with empty input, got %d records", len(result))
		}

		// Test with fewer records than maxSize
		records = make([]Record, 5)
		for i := range records {
			records[i] = Record{Time: time.Now(), Message: "test"}
		}
		result = cleanupFn(records)
		if len(result) != 5 {
			t.Errorf("Expected all 5 records to remain, got %d", len(result))
		}

		// Test with more records than maxSize
		records = make([]Record, 20)
		for i := range records {
			records[i] = Record{Time: time.Now(), Message: "test " + string(rune('a'+i))}
		}
		result = cleanupFn(records)
		if len(result) != 10 {
			t.Errorf("Expected 10 records after cleanup, got %d", len(result))
		}

		// Verify the newest 10 records were kept
		if result[0].Message != "test k" {
			t.Errorf("Expected first record message to be 'test k', got %s", result[0].Message)
		}
	})

	t.Run("MaxAgeCleanup", func(t *testing.T) {
		// Test with empty slice
		records := []Record{}
		cleanupFn := maxAgeCleanup(10 * time.Minute)
		result := cleanupFn(records)
		if len(result) != 0 {
			t.Errorf("Expected empty result with empty input, got %d records", len(result))
		}

		// Test with all records too old
		now := time.Now()
		records = make([]Record, 5)
		for i := range records {
			records[i] = Record{Time: now.Add(-30 * time.Minute), Message: "old"}
		}
		result = cleanupFn(records)
		if len(result) != 0 {
			t.Errorf("Expected 0 records when all are too old, got %d", len(result))
		}

		// Test with all records new enough
		records = make([]Record, 5)
		for i := range records {
			records[i] = Record{Time: now.Add(-5 * time.Minute), Message: "new"}
		}
		result = cleanupFn(records)
		if len(result) != 5 {
			t.Errorf("Expected all 5 records to remain, got %d", len(result))
		}

		// Test with mixed old and new records
		records = make([]Record, 10)
		for i := range records {
			if i < 5 {
				records[i] = Record{Time: now.Add(-30 * time.Minute), Message: "old"}
			} else {
				records[i] = Record{Time: now.Add(-5 * time.Minute), Message: "new"}
			}
		}
		result = cleanupFn(records)
		if len(result) != 5 {
			t.Errorf("Expected 5 records to remain, got %d", len(result))
		}
		for i, r := range result {
			if r.Message != "new" {
				t.Errorf("Record %d should be new, got message %s", i, r.Message)
			}
		}
	})
}

func BenchmarkCleanup_MaxSize(b *testing.B) {
	testCases := []struct {
		initialSize int
		maxSize     int
		numRecords  int
	}{
		{0, 10, 1000},
		{0, 100, 1000},
		{0, 1000, 1000},
		{100, 10, 1000},
		{100, 100, 1000},
		{100, 1000, 1000},
		{1000, 10, 10000},
		{1000, 100, 10000},
		{1000, 1000, 10000},
		{10000, 10, 100000},
		{10000, 100, 100000},
		{10000, 1000, 100000},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("InitialSize_%d_MaxSize_%d_Records_%d",
			tc.initialSize, tc.maxSize, tc.numRecords)

		b.Run(name, func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxSize(tc.maxSize),
						WithAsyncCleanup(false))

					tm := time.Now()
					b.StartTimer()
					for range tc.numRecords {
						store.Append(&Record{
							Time:    tm,
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})
					}
				}
			})

			b.Run("Async", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxSize(tc.maxSize),
						WithAsyncCleanup(true),
						WithContext(b.Context()),
						WithDebounceTime(1*time.Millisecond))

					tm := time.Now()
					b.StartTimer()
					for range tc.numRecords {
						store.Append(&Record{
							Time:    tm,
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})
					}
				}
			})

			b.Run("Sync Concurrent", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxSize(tc.maxSize),
						WithAsyncCleanup(false))

					wg := &sync.WaitGroup{}
					wg.Add(tc.numRecords)
					tm := time.Now()
					ctx, cancel := context.WithCancel(b.Context())
					for range tc.numRecords {
						go func() {
							defer wg.Done()
							<-ctx.Done() // wait for context to be canceled
							store.Append(&Record{
								Time:    tm,
								Level:   slog.LevelInfo,
								Message: "trigger cleanup",
							})
						}()
					}

					b.StartTimer()
					cancel()
					wg.Wait()
				}
			})

			b.Run("Async Concurrent", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxSize(tc.maxSize),
						WithAsyncCleanup(true),
						WithContext(b.Context()),
						WithDebounceTime(1*time.Millisecond))

					wg := &sync.WaitGroup{}
					wg.Add(tc.numRecords)
					tm := time.Now()
					ctx, cancel := context.WithCancel(b.Context())
					for range tc.numRecords {
						go func() {
							defer wg.Done()
							<-ctx.Done() // wait for context to be canceled
							store.Append(&Record{
								Time:    tm,
								Level:   slog.LevelInfo,
								Message: "trigger cleanup",
							})
						}()
					}

					b.StartTimer()
					cancel()
					wg.Wait()
				}
			})
		})
	}
}

func BenchmarkCleanup_MaxAge(b *testing.B) {
	testCases := []struct {
		initialSize int
		maxAge      time.Duration
		numRecords  int
		ageStr      string
	}{
		{0, 10 * time.Millisecond, 100, "10ms"},
		{0, 100 * time.Millisecond, 100, "100ms"},
		{0, 1 * time.Second, 100, "1s"},
		{100, 10 * time.Millisecond, 1000, "10ms"},
		{100, 100 * time.Millisecond, 1000, "100ms"},
		{100, 1 * time.Second, 1000, "1s"},
		{1000, 10 * time.Millisecond, 10000, "10ms"},
		{1000, 100 * time.Millisecond, 10000, "100ms"},
		{1000, 1 * time.Second, 10000, "1s"},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("InitialSize_%d_MaxAge_%s_Records_%d",
			tc.initialSize, tc.ageStr, tc.numRecords)

		b.Run(name, func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxAge(tc.maxAge),
						WithAsyncCleanup(false))

					tm := time.Now()
					b.StartTimer()
					for range tc.numRecords {
						store.Append(&Record{
							Time:    tm,
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})
					}
				}
			})

			b.Run("Async", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, 0,
						WithMaxAge(tc.maxAge),
						WithAsyncCleanup(true),
						WithContext(b.Context()),
						WithDebounceTime(1*time.Millisecond))

					tm := time.Now()
					b.StartTimer()
					for range tc.numRecords {
						store.Append(&Record{
							Time:    tm,
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})
					}
				}
			})

			b.Run("Sync Concurrent", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, 0,
						WithMaxAge(tc.maxAge),
						WithAsyncCleanup(false))

					wg := &sync.WaitGroup{}
					wg.Add(tc.numRecords)
					tm := time.Now()
					ctx, cancel := context.WithCancel(b.Context())
					for range tc.numRecords {
						go func() {
							defer wg.Done()
							<-ctx.Done()
							store.Append(&Record{
								Time:    tm,
								Level:   slog.LevelInfo,
								Message: "trigger cleanup",
							})
						}()
					}

					b.StartTimer()
					cancel()
					wg.Wait()
				}
			})

			b.Run("Async Concurrent", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxAge(tc.maxAge),
						WithAsyncCleanup(true),
						WithContext(b.Context()),
						WithDebounceTime(1*time.Millisecond))

					wg := &sync.WaitGroup{}
					wg.Add(tc.numRecords)
					tm := time.Now()
					ctx, cancel := context.WithCancel(b.Context())
					for range tc.numRecords {
						go func() {
							defer wg.Done()
							<-ctx.Done()
							store.Append(&Record{
								Time:    tm,
								Level:   slog.LevelInfo,
								Message: "trigger cleanup",
							})
						}()
					}

					b.StartTimer()
					cancel()
					wg.Wait()
				}
			})
		})
	}
}

func BenchmarkCleanup_MixedWorkload(b *testing.B) {
	testCases := []struct {
		initialSize int
		maxSize     int
		numRecords  int
		readRatio   float64
	}{
		{100, 50, 1000, 0.5},
		{100, 50, 1000, 0.9},
		{1000, 500, 1000, 0.5},
		{1000, 500, 1000, 0.9},
		{10000, 5000, 10000, 0.5},
		{10000, 5000, 10000, 0.9},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("InitialSize_%d_MaxSize_%d_Records_%d_ReadRatio_%.0f",
			tc.initialSize, tc.maxSize, tc.numRecords, tc.readRatio*100)

		b.Run(name, func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxSize(tc.maxSize),
						WithAsyncCleanup(false))

					wg := &sync.WaitGroup{}
					wg.Add(tc.numRecords)
					tm := time.Now()
					ctx, cancel := context.WithCancel(b.Context())
					for i := range tc.numRecords {
						go func() {
							defer wg.Done()
							<-ctx.Done()
							if float64(i%100)/100 < tc.readRatio {
								_ = store.GetAll()
							} else {
								store.Append(&Record{
									Time:    tm,
									Level:   slog.LevelInfo,
									Message: "mixed workload",
								})
							}
						}()
					}

					b.StartTimer()
					cancel()
					wg.Wait()
				}
			})

			b.Run("Async", func(b *testing.B) {
				for b.Loop() {
					b.StopTimer()
					store, _ := setupTestStorage(b, tc.initialSize,
						WithMaxSize(tc.maxSize),
						WithAsyncCleanup(true),
						WithContext(b.Context()),
						WithDebounceTime(1*time.Millisecond))

					wg := &sync.WaitGroup{}
					wg.Add(tc.numRecords)
					tm := time.Now()
					ctx, cancel := context.WithCancel(b.Context())
					for i := range tc.numRecords {
						go func() {
							defer wg.Done()
							<-ctx.Done()
							if float64(i%100)/100 < tc.readRatio {
								_ = store.GetAll()
							} else {
								store.Append(&Record{
									Time:    tm,
									Level:   slog.LevelInfo,
									Message: "mixed workload",
								})
							}
						}()
					}

					b.StartTimer()
					cancel()
					wg.Wait()
				}
			})
		})
	}
}
