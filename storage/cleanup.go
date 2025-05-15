package storage

import "time"

// CleanupFunc defines a function signature for cleanup operations
type CleanupFunc func(records []Record) []Record

// maxSizeCleanup creates a cleanup function that limits the number of records
// by removing the oldest entries when the maximum size is exceeded
func maxSizeCleanup(maxSize int) CleanupFunc {
	return func(records []Record) []Record {
		if len(records) <= maxSize {
			return records
		}
		// Keep only the most recent records up to maxSize
		return records[len(records)-maxSize:]
	}
}

// maxAgeCleanup creates a cleanup function that removes records older than the specified duration
func maxAgeCleanup(maxAge time.Duration) CleanupFunc {
	return func(records []Record) []Record {
		if len(records) == 0 {
			return records
		}

		cutoff := time.Now().Add(-maxAge)

		// Find the index of the first record to keep
		i := 0
		for ; i < len(records); i++ {
			if records[i].Time.After(cutoff) {
				break
			}
		}

		// If all records are too old, return empty slice
		if i >= len(records) {
			return records[:0]
		}

		// If no records are too old, return all records
		if i == 0 {
			return records
		}

		// Remove old records
		return records[i:]
	}
}
