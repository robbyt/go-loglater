package storage

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Storage is an interface for a storage backend
type Storage interface {
	Append(record *Record)
	GetAll() []Record
}

// RecordStorage holds the shared log records
type RecordStorage struct {
	mu                  sync.RWMutex
	records             []Record
	cleanupFunc         CleanupFunc
	asyncCleanupEnabled bool
	cleanupDebounce     time.Duration

	cleanupCh           chan struct{}
	ctx                 context.Context
	asyncCleanupRunning atomic.Bool
}

// NewRecordStorage creates a new RecordStorage instance
func NewRecordStorage(opts ...Option) *RecordStorage {
	rs := &RecordStorage{
		records:         make([]Record, 0, 10), // Default preallocation size of 10
		cleanupCh:       make(chan struct{}, 1),
		ctx:             context.Background(),
		cleanupDebounce: 10 * time.Second,
	}

	// Apply all functional options
	for _, opt := range opts {
		opt(rs)
	}

	// Start a background worker for cleanup if enabled
	if rs.asyncCleanupEnabled {
		go rs.StartCleanupWorker()
	}

	return rs
}

// StartCleanupWorker handles async cleanup operations in a go routine
func (s *RecordStorage) StartCleanupWorker() {
	if !s.asyncCleanupRunning.CompareAndSwap(false, true) {
		// Already running, exit
		return
	}
	defer s.asyncCleanupRunning.Store(false)

	timer := time.NewTimer(s.cleanupDebounce)
	timer.Stop() // Stop immediately as we don't want to trigger right away

	for {
		select {
		case <-s.cleanupCh:
			// Reset timer on new cleanup request
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(s.cleanupDebounce)

		case <-timer.C:
			// Perform the cleanup after debounce period
			s.performCleanup()

		case <-s.ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

// performCleanup executes the cleanup function if set
func (s *RecordStorage) performCleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cleanupFunc != nil && len(s.records) > 0 {
		s.records = s.cleanupFunc(s.records)
	}
}

// triggerCleanup triggers a cleanup operation
func (s *RecordStorage) triggerCleanup() {
	if !s.asyncCleanupEnabled {
		s.performCleanup()
		return
	}

	// Non-blocking send to trigger async cleanup
	select {
	case s.cleanupCh <- struct{}{}:
	default:
		// Channel is full, cleanup is already scheduled
	}
}

// Append adds a record to the storage
func (s *RecordStorage) Append(record *Record) {
	s.mu.Lock()
	s.records = append(s.records, *record)
	s.mu.Unlock()

	// Trigger cleanup after append
	if s.cleanupFunc != nil {
		s.triggerCleanup()
	}
}

// GetAll returns a copy of all records
func (s *RecordStorage) GetAll() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.records)
}
