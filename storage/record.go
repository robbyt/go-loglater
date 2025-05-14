package storage

import (
	"context"
	"log/slog"
	"time"
)

// Record represents a log Record that can be stored
type Record struct {
	Time    time.Time
	Level   slog.Level
	Message string
	Attrs   []slog.Attr
	Groups  []string
}

func NewRecord(ctx context.Context, groups []string, r slog.Record) *Record {
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
