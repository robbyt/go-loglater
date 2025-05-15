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
}

// NewLogCollector creates a new log collector with an underlying handler and optional configuration
func NewLogCollector(baseHandler slog.Handler, opts ...Option) *LogCollector {
	lc := &LogCollector{
		store:   storage.NewRecordStorage(),
		handler: baseHandler,
		groups:  make([]string, 0),
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

	// Store the record in the shared store
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

	// Create a deep copy of the groups slice
	groupsCopy := slices.Clone(c.groups)

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:   c.store,
		handler: newHandler,
		groups:  groupsCopy,
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

	// Create a new collector that shares the same record store
	return &LogCollector{
		store:   c.store,
		handler: newHandler,
		groups:  newGroups,
	}
}

// PlayLogs outputs all stored logs to the provided handler
func (c *LogCollector) PlayLogs(handler slog.Handler) error {
	if c.store == nil {
		return nil
	}

	for _, stored := range c.store.GetAll() {
		// Apply groups from the stored record
		currentHandler := handler
		for _, group := range stored.Groups {
			currentHandler = currentHandler.WithGroup(group)
		}

		// Create a new record from the stored data
		r := slog.NewRecord(stored.Time, stored.Level, stored.Message, 0)
		for _, attr := range stored.Attrs {
			r.AddAttrs(attr)
		}

		if err := currentHandler.Handle(context.Background(), r); err != nil {
			return err
		}
	}
	return nil
}

// GetLogs returns a copy of the collected logs
func (c *LogCollector) GetLogs() []storage.Record {
	if c.store == nil {
		return nil
	}

	// Create a copy to avoid exposing the internal slice
	result := make([]storage.Record, len(c.store.GetAll()))
	copy(result, c.store.GetAll())
	return result
}
