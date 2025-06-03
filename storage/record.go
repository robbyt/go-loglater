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
	PC      uintptr // Program counter for call site information
	Attrs   []slog.Attr
	Journal OperationJournal // journal of handler operations for replay
}

// NewRecord creates a new Record from a slog.Record and journal.
//
// The journal parameter captures the exact order of WithAttrs() and WithGroup() operations
// that were used to create the logger instance that generated this log record.
func NewRecord(_ context.Context, journal OperationJournal, r *slog.Record) *Record {
	if r == nil {
		return nil
	}

	record := &Record{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
		PC:      r.PC, // Preserve program counter for call site information
		Attrs:   make([]slog.Attr, 0, r.NumAttrs()),
		Journal: journal,
	}

	// Extract attributes
	r.Attrs(func(attr slog.Attr) bool {
		record.Attrs = append(record.Attrs, attr)
		return true
	})

	return record
}

// Realize returns a new Record with all attributes from the journal applied.
func (r *Record) Realize() Record {
	result := Record{
		Time:    r.Time,
		Level:   r.Level,
		Message: r.Message,
		PC:      r.PC,
		Attrs:   make([]slog.Attr, 0),
		Journal: r.Journal,
	}

	// Apply the journal to build the attributes
	var currentGroups []string
	var collectorAttrs []slog.Attr

	for _, op := range r.Journal {
		switch op.Type {
		case OpAttrs:
			if len(currentGroups) > 0 {
				// These attributes belong to the current group
				for _, attr := range op.Attrs {
					collectorAttrs = append(collectorAttrs, applyGroups(attr, currentGroups))
				}
			} else {
				// Global attributes
				collectorAttrs = append(collectorAttrs, op.Attrs...)
			}
		case OpGroup:
			currentGroups = append(currentGroups, op.Group)
		}
	}

	// First add collector attributes (from WithAttrs)
	result.Attrs = append(result.Attrs, collectorAttrs...)

	// Then add record attributes (from the log message itself)
	// These need to be grouped based on the final group state
	for _, attr := range r.Attrs {
		if len(currentGroups) > 0 {
			result.Attrs = append(result.Attrs, applyGroups(attr, currentGroups))
		} else {
			result.Attrs = append(result.Attrs, attr)
		}
	}

	return result
}

// applyGroups creates a new attribute with groups applied as nested structure
func applyGroups(attr slog.Attr, groups []string) slog.Attr {
	if len(groups) == 0 {
		return attr
	}

	// Build nested groups from the inside out
	result := attr
	for i := len(groups) - 1; i >= 0; i-- {
		result = slog.Group(groups[i], result)
	}

	return result
}
