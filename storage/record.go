package storage

import (
	"context"
	"log/slog"
	"time"
)

// Record represents a log Record that can be stored, somewhere.
type Record struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   []slog.Attr
	Groups  []string
}

// NewRecord creates a new Record from a slog.Record and an optional list of groups (for WithGroup namespacing).
func NewRecord(_ context.Context, groups []string, r *slog.Record) *Record {
	if r == nil {
		return nil
	}

	record := &Record{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
		Attrs:   make([]slog.Attr, 0, r.NumAttrs()),
		Groups:  groups,
	}

	// Extract attributes
	r.Attrs(func(attr slog.Attr) bool {
		record.Attrs = append(record.Attrs, attr)
		return true
	})

	return record
}
