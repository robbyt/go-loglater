package storage

import (
	"context"
	"time"
)

// Option defines a function type for configuring RecordStorage
type Option func(*MemStorage)

// WithPreallocation sets the initial capacity for the record storage.
func WithPreallocation(size int) Option {
	return func(rs *MemStorage) {
		rs.records = make([]Record, 0, size)
	}
}

// WithAsyncCleanup enables or disables asynchronous cleanup for records.
func WithAsyncCleanup(enabled bool) Option {
	return func(rs *MemStorage) {
		rs.asyncCleanupEnabled = enabled
	}
}

// WithMaxSize sets a maximum size for the record store, removing oldest records when exceeded.
func WithMaxSize(maxSize int) Option {
	return WithCleanupFunc(maxSizeCleanup(maxSize))
}

// WithMaxAge sets a maximum age for records, removing them when exceeded.
func WithMaxAge(maxAge time.Duration) Option {
	return WithCleanupFunc(maxAgeCleanup(maxAge))
}

// WithCleanupFunc allows setting a custom cleanup function.
func WithCleanupFunc(cleanupFn CleanupFunc) Option {
	return func(rs *MemStorage) {
		rs.cleanupFunc = cleanupFn
	}
}

// WithContext sets a context for controlling the async cleanup worker.
// The worker will exit when the context is canceled.
func WithContext(ctx context.Context) Option {
	return func(rs *MemStorage) {
		if ctx != nil {
			rs.ctx = ctx
		}
	}
}

// WithDebounceTime sets the debounce time for async cleanup operations.
// This controls how frequently async cleanup operations are triggered.
// Default is 10 seconds.
func WithDebounceTime(duration time.Duration) Option {
	return func(rs *MemStorage) {
		if duration > 0 {
			rs.cleanupDebounce = duration
		}
	}
}
