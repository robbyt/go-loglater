package storage

import (
	"context"
	"log/slog"
	"time"
)

// Operation represents a single handler modification operation.
// This captures either a WithAttrs() or WithGroup() call made on a slog.Handler,
// preserving the exact order of operations for accurate replay.
//
// Example sequence for: logger.With("global", "value").WithGroup("api").With("user", "123")
//  1. Operation{Type: "attrs", Attrs: [global=value]}
//  2. Operation{Type: "group", Group: "api"}
//  3. Operation{Type: "attrs", Attrs: [user=123]}
//
// During replay, this ensures "global" stays global while "user" gets grouped as "api.user".
type Operation struct {
	Type  string      // "attrs" or "group"
	Attrs []slog.Attr // for WithAttrs operations
	Group string      // for WithGroup operations
}

// HandlerSequence represents the complete sequence of WithAttrs and WithGroup operations
// that were applied to create a particular logger instance.
//
// This sequence is stored alongside each log record and replayed during log output
// to ensure the exact same handler state is reconstructed, preserving the correct
// relationship between global attributes (added before groups) and grouped attributes
// (added after groups).
//
// Without this sequence tracking, attributes could be incorrectly grouped during replay,
// causing "global=value" to become "group.global=value".
type HandlerSequence []Operation

// Record represents a log Record that can be stored, somewhere.
type Record struct {
	Time     time.Time
	Level    slog.Level
	Message  string
	PC       uintptr // Program counter for call site information
	Attrs    []slog.Attr
	Sequence HandlerSequence // Sequence of handler operations for accurate replay
}

// NewRecord creates a new Record from a slog.Record and handler sequence.
//
// The sequence parameter captures the exact order of WithAttrs() and WithGroup() operations
// that were used to create the logger instance that generated this log record. This sequence
// is essential for accurate replay of the log with correct attribute grouping.
func NewRecord(_ context.Context, sequence HandlerSequence, r *slog.Record) *Record {
	if r == nil {
		return nil
	}

	record := &Record{
		Time:     r.Time,
		Level:    r.Level,
		Message:  r.Message,
		PC:       r.PC, // Preserve program counter for call site information
		Attrs:    make([]slog.Attr, 0, r.NumAttrs()),
		Sequence: sequence,
	}

	// Extract attributes
	r.Attrs(func(attr slog.Attr) bool {
		record.Attrs = append(record.Attrs, attr)
		return true
	})

	return record
}

// Realize returns a new Record with all attributes from the sequence applied.
// This includes both collector attributes (from WithAttrs) and message attributes,
// with proper group nesting applied.
func (r *Record) Realize() Record {
	result := Record{
		Time:     r.Time,
		Level:    r.Level,
		Message:  r.Message,
		PC:       r.PC,
		Attrs:    make([]slog.Attr, 0),
		Sequence: r.Sequence,
	}

	// Apply the sequence to build complete attributes
	var currentGroups []string
	var collectorAttrs []slog.Attr

	for _, op := range r.Sequence {
		switch op.Type {
		case "attrs":
			if len(currentGroups) > 0 {
				// These attributes belong to the current group
				for _, attr := range op.Attrs {
					collectorAttrs = append(collectorAttrs, applyGroups(attr, currentGroups))
				}
			} else {
				// Global attributes
				collectorAttrs = append(collectorAttrs, op.Attrs...)
			}
		case "group":
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
