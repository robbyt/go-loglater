package storage

import (
	"slices"
	"sync"
)

// Storage is an interface for a storage backend
type Storage interface {
	Append(record *Record)
	GetAll() []Record
}

// RecordStorage holds the shared log records
type RecordStorage struct {
	mu      sync.RWMutex
	records []Record
}

func NewRecordStorage(size int) *RecordStorage {
	return &RecordStorage{
		records: make([]Record, 0, size),
	}
}

func (s *RecordStorage) Append(record *Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, *record)
}

func (s *RecordStorage) GetAll() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.records)
}
