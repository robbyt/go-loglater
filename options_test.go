package loglater

import (
	"testing"

	"github.com/robbyt/go-loglater/storage"
)

func TestWithStorage(t *testing.T) {
	// Create a custom storage implementation
	customStore := storage.NewRecordStorage(storage.WithPreallocation(5))

	// Create a collector with the custom storage
	collector := NewLogCollector(nil, WithStorage(customStore))

	// Verify that the collector is using our custom storage
	if collector.store != customStore {
		t.Errorf("WithStorage option did not set the storage correctly")
	}
}
