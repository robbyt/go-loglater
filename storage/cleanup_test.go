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

	for i := 0; i < size; i++ {
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
	testSizes := []int{100, 1000, 10000}
	maxSizes := []int{10, 100, 1000}

	for _, size := range testSizes {
		sizeName := fmt.Sprintf("Size_%d", size)
		b.Run(sizeName, func(b *testing.B) {
			for _, maxSize := range maxSizes {
				retention := int(float64(maxSize) / float64(size) * 100)

				// Skip unreasonable combinations
				if maxSize > size {
					continue
				}

				// Benchmark synchronous cleanup
				testName := fmt.Sprintf("Sync_MaxSize_%d_%dpct", maxSize, retention)
				b.Run(testName, func(b *testing.B) {
					// Use very short debounce time for benchmarking
					store, _ := setupTestStorage(b, size,
						WithMaxSize(maxSize),
						WithAsyncCleanup(false))

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						// Add a new record to trigger cleanup
						store.Append(&Record{
							Time:    time.Now(),
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})
					}
				})

				// Benchmark asynchronous cleanup
				testName = fmt.Sprintf("Async_MaxSize_%d_%dpct", maxSize, retention)
				b.Run(testName, func(b *testing.B) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					// Use very short debounce time for benchmarking
					store, _ := setupTestStorage(b, size,
						WithMaxSize(maxSize),
						WithAsyncCleanup(true),
						WithContext(ctx),
						WithDebounceTime(1*time.Millisecond))

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						// Add a new record to trigger cleanup
						store.Append(&Record{
							Time:    time.Now(),
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})

						// For async we need to ensure the cleanup has a chance to run
						if i%100 == 0 {
							time.Sleep(2 * time.Millisecond)
						}
					}
				})
			}
		})
	}
}

func BenchmarkCleanup_MaxAge(b *testing.B) {
	testSizes := []int{100, 1000, 10000}
	maxAges := []time.Duration{10 * time.Second, 30 * time.Second, 5 * time.Minute}

	for _, size := range testSizes {
		sizeName := fmt.Sprintf("Size_%d", size)
		b.Run(sizeName, func(b *testing.B) {
			for _, maxAge := range maxAges {
				// Calculate approximate retention percentage (for test naming)
				retention := int(float64(maxAge.Seconds()) / float64(size) * 100)
				if retention > 100 {
					retention = 100
				}

				// Benchmark synchronous cleanup
				ageName := maxAge.String()
				testName := fmt.Sprintf("Sync_MaxAge_%s_%dpct", ageName, retention)
				b.Run(testName, func(b *testing.B) {
					store, _ := setupTestStorage(b, size,
						WithMaxAge(maxAge),
						WithAsyncCleanup(false))

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						// Add a new record to trigger cleanup
						store.Append(&Record{
							Time:    time.Now(),
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})
					}
				})

				// Benchmark asynchronous cleanup
				testName = fmt.Sprintf("Async_MaxAge_%s_%dpct", ageName, retention)
				b.Run(testName, func(b *testing.B) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					// Use very short debounce time for benchmarking
					store, _ := setupTestStorage(b, size,
						WithMaxAge(maxAge),
						WithAsyncCleanup(true),
						WithContext(ctx),
						WithDebounceTime(1*time.Millisecond))

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						// Add a new record to trigger cleanup
						store.Append(&Record{
							Time:    time.Now(),
							Level:   slog.LevelInfo,
							Message: "trigger cleanup",
						})

						// For async we need to ensure the cleanup has a chance to run
						if i%100 == 0 {
							time.Sleep(2 * time.Millisecond)
						}
					}
				})
			}
		})
	}
}

func BenchmarkCleanup_HighLoad(b *testing.B) {
	// Test high-concurrency scenarios
	concurrencyLevels := []int{10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		concName := fmt.Sprintf("Concurrency_%d", concurrency)

		// Test MaxSize with high concurrency
		b.Run("MaxSize_"+concName, func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				store, _ := setupTestStorage(b, 1000,
					WithMaxSize(100),
					WithAsyncCleanup(false))

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var wg sync.WaitGroup
					wg.Add(concurrency)

					for j := 0; j < concurrency; j++ {
						go func() {
							store.Append(&Record{
								Time:    time.Now(),
								Level:   slog.LevelInfo,
								Message: "concurrent append",
							})
							wg.Done()
						}()
					}

					wg.Wait()
				}
			})

			b.Run("Async", func(b *testing.B) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				store, _ := setupTestStorage(b, 1000,
					WithMaxSize(100),
					WithAsyncCleanup(true),
					WithContext(ctx),
					WithDebounceTime(1*time.Millisecond))

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var wg sync.WaitGroup
					wg.Add(concurrency)

					for j := 0; j < concurrency; j++ {
						go func() {
							store.Append(&Record{
								Time:    time.Now(),
								Level:   slog.LevelInfo,
								Message: "concurrent append",
							})
							wg.Done()
						}()
					}

					wg.Wait()

					// Give cleanup a chance to run
					if i%10 == 0 {
						time.Sleep(2 * time.Millisecond)
					}
				}
			})
		})

		// Test MaxAge with high concurrency
		b.Run("MaxAge_"+concName, func(b *testing.B) {
			b.Run("Sync", func(b *testing.B) {
				store, _ := setupTestStorage(b, 1000,
					WithMaxAge(30*time.Second),
					WithAsyncCleanup(false))

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var wg sync.WaitGroup
					wg.Add(concurrency)

					for j := 0; j < concurrency; j++ {
						go func() {
							store.Append(&Record{
								Time:    time.Now(),
								Level:   slog.LevelInfo,
								Message: "concurrent append",
							})
							wg.Done()
						}()
					}

					wg.Wait()
				}
			})

			b.Run("Async", func(b *testing.B) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				store, _ := setupTestStorage(b, 1000,
					WithMaxAge(30*time.Second),
					WithAsyncCleanup(true),
					WithContext(ctx),
					WithDebounceTime(1*time.Millisecond))

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var wg sync.WaitGroup
					wg.Add(concurrency)

					for j := 0; j < concurrency; j++ {
						go func() {
							store.Append(&Record{
								Time:    time.Now(),
								Level:   slog.LevelInfo,
								Message: "concurrent append",
							})
							wg.Done()
						}()
					}

					wg.Wait()

					// Give cleanup a chance to run
					if i%10 == 0 {
						time.Sleep(2 * time.Millisecond)
					}
				}
			})
		})
	}
}

// BenchmarkCleanup_MixedWorkload tests a mixed workload of append and retrieve operations
func BenchmarkCleanup_MixedWorkload(b *testing.B) {
	readRatio := 0.7 // 70% reads, 30% writes

	// Test with both cleanup strategies
	b.Run("MaxSize", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			store, _ := setupTestStorage(b, 1000,
				WithMaxSize(500),
				WithAsyncCleanup(false))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if float64(i%100)/100 < readRatio {
					// Read operation
					_ = store.GetAll()
				} else {
					// Write operation - triggers sync cleanup
					store.Append(&Record{
						Time:    time.Now(),
						Level:   slog.LevelInfo,
						Message: "mixed workload",
					})
				}
			}
		})

		b.Run("Async", func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			store, _ := setupTestStorage(b, 1000,
				WithMaxSize(500),
				WithAsyncCleanup(true),
				WithContext(ctx),
				WithDebounceTime(1*time.Millisecond))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if float64(i%100)/100 < readRatio {
					// Read operation
					_ = store.GetAll()
				} else {
					// Write operation - triggers async cleanup
					store.Append(&Record{
						Time:    time.Now(),
						Level:   slog.LevelInfo,
						Message: "mixed workload",
					})
				}

				// Occasionally let async worker run
				if i%1000 == 0 {
					time.Sleep(2 * time.Millisecond)
				}
			}
		})
	})

	b.Run("MaxAge", func(b *testing.B) {
		b.Run("Sync", func(b *testing.B) {
			store, _ := setupTestStorage(b, 1000,
				WithMaxAge(30*time.Second),
				WithAsyncCleanup(false))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if float64(i%100)/100 < readRatio {
					// Read operation
					_ = store.GetAll()
				} else {
					// Write operation - triggers sync cleanup
					store.Append(&Record{
						Time:    time.Now(),
						Level:   slog.LevelInfo,
						Message: "mixed workload",
					})
				}
			}
		})

		b.Run("Async", func(b *testing.B) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			store, _ := setupTestStorage(b, 1000,
				WithMaxAge(30*time.Second),
				WithAsyncCleanup(true),
				WithContext(ctx),
				WithDebounceTime(1*time.Millisecond))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if float64(i%100)/100 < readRatio {
					// Read operation
					_ = store.GetAll()
				} else {
					// Write operation - triggers async cleanup
					store.Append(&Record{
						Time:    time.Now(),
						Level:   slog.LevelInfo,
						Message: "mixed workload",
					})
				}

				// Occasionally let async worker run
				if i%1000 == 0 {
					time.Sleep(2 * time.Millisecond)
				}
			}
		})
	})
}
