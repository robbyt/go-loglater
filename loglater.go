// Package loglater provides a slog.Handler that captures logs for later replay.
//
//	collector := NewLogCollector(nil)
//	logger := slog.New(collector)
//
//	logger.Info("user login", "user_id", 123)
//
//	// Inspect captured logs
//	logs := collector.GetLogs()
//	fmt.Println(logs[0].Message) // "user login"
//
//	// Replay to any handler
//	collector.PlayLogs(slog.NewJSONHandler(os.Stdout, nil))
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
	store   Storage
	handler slog.Handler
	journal storage.OperationJournal
}

// NewLogCollector creates a new log collector with an underlying handler and optional configuration
func NewLogCollector(baseHandler slog.Handler, opts ...Option) *LogCollector {
	lc := &LogCollector{
		store:   storage.NewRecordStorage(),
		handler: baseHandler,
		journal: make(storage.OperationJournal, 0),
	}

	// Apply all options
	for _, opt := range opts {
		opt(lc)
	}

	return lc
}

// Handle implements slog.Handler.Handle
func (c *LogCollector) Handle(ctx context.Context, r slog.Record) error {
	journalCopy := slices.Clone(c.journal)
	storedRecord := storage.NewRecord(ctx, journalCopy, &r)
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

	// Add the WithGroup operation to the new copy of the journal
	journalCopy := slices.Clone(c.journal)
	journalCopy = append(journalCopy, storage.Operation{
		Type:  storage.OpAttrs,
		Attrs: attrs,
	})

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:   c.store,
		handler: newHandler,
		journal: journalCopy,
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

	// Add the WithGroup operation to the new copy of the journal
	journalCopy := slices.Clone(c.journal)
	journalCopy = append(journalCopy, storage.Operation{
		Type:  storage.OpGroup,
		Group: name,
	})

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:   c.store,
		handler: newHandler,
		journal: journalCopy,
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

		// Replay the journal of WithAttrs/WithGroup operations
		for _, op := range stored.Journal {
			switch op.Type {
			case storage.OpAttrs:
				currentHandler = currentHandler.WithAttrs(op.Attrs)
			case storage.OpGroup:
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
// Each returned record contains the same attributes that would be present during replay.
func (c *LogCollector) GetLogs() []storage.Record {
	// Get raw records and realize them for the user
	rawRecords := c.store.GetAll()
	realizedRecords := make([]storage.Record, len(rawRecords))

	for i, record := range rawRecords {
		realizedRecords[i] = record.Realize()
	}

	return realizedRecords
}
