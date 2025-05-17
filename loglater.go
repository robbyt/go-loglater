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
	groups  []string
	attrs   []slog.Attr
}

// NewLogCollector creates a new log collector with an underlying handler and optional configuration
func NewLogCollector(baseHandler slog.Handler, opts ...Option) *LogCollector {
	lc := &LogCollector{
		store:   storage.NewRecordStorage(),
		handler: baseHandler,
		groups:  make([]string, 0),
		attrs:   make([]slog.Attr, 0),
	}

	// Apply all options
	for _, opt := range opts {
		opt(lc)
	}

	return lc
}

// Handle implements slog.Handler.Handle
func (c *LogCollector) Handle(ctx context.Context, r slog.Record) error {
	g := slices.Clone(c.groups)
	storedRecord := storage.NewRecord(ctx, g, &r)
	if storedRecord == nil {
		return errors.New("failed to create record")
	}

	// Add the collector's attributes to the stored record, added via WithAttrs()
	if len(c.attrs) > 0 {
		storedRecord.Attrs = append(storedRecord.Attrs, c.attrs...)
	}

	if c.store != nil {
		c.store.Append(storedRecord)
	}

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

	// Create a deep copy of the groups and attrs slices
	groupsCopy := slices.Clone(c.groups)
	attrsCopy := slices.Clone(c.attrs)

	// Append the new attributes
	attrsCopy = append(attrsCopy, attrs...)

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:   c.store,
		handler: newHandler,
		groups:  groupsCopy,
		attrs:   attrsCopy,
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

	// Copy existing groups and append the new group
	newGroups := slices.Clone(c.groups)
	newGroups = append(newGroups, name)

	// Copy existing attributes
	newAttrs := slices.Clone(c.attrs)

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:   c.store,
		handler: newHandler,
		groups:  newGroups,
		attrs:   newAttrs,
	}
}

// PlayLogsCtx outputs all stored logs to the provided handler with context support
func (c *LogCollector) PlayLogsCtx(ctx context.Context, handler slog.Handler) error {
	if c.store == nil {
		return nil
	}

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

		// Apply groups from the stored records
		for _, group := range stored.Groups {
			currentHandler = currentHandler.WithGroup(group)
		}

		// Create a new record from the stored data
		r := slog.NewRecord(stored.Time, stored.Level, stored.Message, 0)
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

// GetLogs returns a copy of the collected logs
func (c *LogCollector) GetLogs() []storage.Record {
	if c.store == nil {
		return nil
	}
	return c.store.GetAll()
}
