// Package loglater provides a slog.Handler implementation that captures structured logs
// for later replay, enabling powerful testing and debugging capabilities.
//
// Why This Package Exists:
//
// Testing code that uses structured logging is challenging. You want to verify that
// your code logs the right messages with the right attributes, but you also want
// those logs to be available for debugging when tests fail. This package solves
// both needs by capturing logs during execution and allowing you to replay them
// to any slog.Handler later.
//
// The Temporal Ordering Challenge:
//
// A subtle but critical challenge in log replay is preserving the exact relationship
// between attributes and groups. Consider this logger:
//
//	logger.With("global", "value").WithGroup("api").With("user", "123")
//
// When replaying, "global" must remain a top-level attribute while "user" must be
// grouped under "api". The naive approach of storing attributes and groups separately
// fails because it loses the temporal ordering of operations.
//
// Our Solution - Operation Sequences:
//
// We record every WithAttrs() and WithGroup() call as an ordered sequence of operations.
// During replay, we execute these operations in the exact same order, reconstructing
// the precise handler state that existed when the log was created. This ensures that
// attributes added before groups remain global, while attributes added after groups
// are properly nested.
//
// Usage:
//
// GetLogs() returns fully realized log records with all attributes and groups applied,
// exactly as they would appear during replay. This includes both the log message's own
// attributes and any attributes added via WithAttrs(), with proper group nesting.
package loglater

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/robbyt/go-loglater/storage"
)

// StorageWriter writes log records to a storage backend
type StorageWriter interface {
	Append(record *storage.Record)
}

// StorageReader returns ALL log records from a storage backend
type StorageReader interface {
	GetAll() []storage.Record
}

// Storage is the full interface for a storage backend
type Storage interface {
	StorageWriter
	StorageReader
}

// LogCollector collects log records and can replay them later
type LogCollector struct {
	store    Storage
	handler  slog.Handler
	sequence storage.HandlerSequence
}

// NewLogCollector creates a new log collector with an underlying handler and optional configuration
func NewLogCollector(baseHandler slog.Handler, opts ...Option) *LogCollector {
	lc := &LogCollector{
		store:    storage.NewRecordStorage(),
		handler:  baseHandler,
		sequence: make(storage.HandlerSequence, 0),
	}

	// Apply all options
	for _, opt := range opts {
		opt(lc)
	}

	return lc
}

// Handle implements slog.Handler.Handle
func (c *LogCollector) Handle(ctx context.Context, r slog.Record) error {
	seq := slices.Clone(c.sequence)
	storedRecord := storage.NewRecord(ctx, seq, &r)
	if storedRecord == nil {
		return errors.New("failed to create record")
	}

	c.store.Append(storedRecord)

	// Forward to underlying handler if it exists
	if c.handler != nil {
		return c.handler.Handle(ctx, r)
	}
	return nil
}

// Enabled implements slog.Handler.Enabled
func (c *LogCollector) Enabled(ctx context.Context, level slog.Level) bool {
	if c.handler == nil {
		return true
	}
	return c.handler.Enabled(ctx, level)
}

// WithAttrs implements slog.Handler.WithAttrs
func (c *LogCollector) WithAttrs(attrs []slog.Attr) slog.Handler {
	// If there are no attrs, return the original handler
	if len(attrs) == 0 {
		return c
	}

	// Create a new handler with the underlying handler (if any)
	var newHandler slog.Handler
	if c.handler != nil {
		newHandler = c.handler.WithAttrs(attrs)
	}

	// Clone sequence to avoid mutation between handler instances
	sequenceCopy := slices.Clone(c.sequence)

	// Add the WithAttrs operation to the sequence
	sequenceCopy = append(sequenceCopy, storage.Operation{
		Type:  "attrs",
		Attrs: attrs,
	})

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:    c.store,
		handler:  newHandler,
		sequence: sequenceCopy,
	}
}

// WithGroup implements slog.Handler.WithGroup
func (c *LogCollector) WithGroup(name string) slog.Handler {
	// If name is empty, return the receiver (matches standard library behavior)
	if name == "" {
		return c
	}

	// Forward to the underlying handler (if any)
	var newHandler slog.Handler
	if c.handler != nil {
		newHandler = c.handler.WithGroup(name)
	}

	// Clone sequence to avoid mutation between handler instances
	sequenceCopy := slices.Clone(c.sequence)

	// Add the WithGroup operation to the sequence
	sequenceCopy = append(sequenceCopy, storage.Operation{
		Type:  "group",
		Group: name,
	})

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:    c.store,
		handler:  newHandler,
		sequence: sequenceCopy,
	}
}

// PlayLogsCtx outputs all stored logs to the provided handler with context support
func (c *LogCollector) PlayLogsCtx(ctx context.Context, handler slog.Handler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	for _, stored := range c.store.GetAll() {
		select {
		case <-ctx.Done():
			// handle context cancellation between log entries
			return ctx.Err()
		default:
			// continue processing
		}

		currentHandler := handler

		// Replay the exact sequence of WithAttrs/WithGroup operations
		for _, op := range stored.Sequence {
			switch op.Type {
			case "attrs":
				currentHandler = currentHandler.WithAttrs(op.Attrs)
			case "group":
				currentHandler = currentHandler.WithGroup(op.Group)
			}
		}

		// Create a new record from the stored data, preserving the original PC
		r := slog.NewRecord(stored.Time, stored.Level, stored.Message, stored.PC)
		for _, attr := range stored.Attrs {
			r.AddAttrs(attr)
		}

		// Forward to the new handler from this function's input
		if err := currentHandler.Handle(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// PlayLogs outputs all stored logs to the provided handler using a background context
func (c *LogCollector) PlayLogs(handler slog.Handler) error {
	return c.PlayLogsCtx(context.Background(), handler)
}

// GetLogs returns a copy of the collected logs with all attributes and groups applied.
// Each returned record contains the complete set of attributes that would be present
// during replay, including attributes from WithAttrs calls and proper group nesting.
func (c *LogCollector) GetLogs() []storage.Record {
	// Get raw records and realize them for the user
	rawRecords := c.store.GetAll()
	realizedRecords := make([]storage.Record, len(rawRecords))

	for i, record := range rawRecords {
		realizedRecords[i] = record.Realize()
	}

	return realizedRecords
}
