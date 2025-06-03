package loglater

import (
	"log/slog"
	"testing"
	"time"

	"github.com/robbyt/go-loglater/storage"
)

// TestGetLogsReturnsCompleteRecords verifies that GetLogs() returns fully realized
// records with all attributes and groups applied
func TestGetLogsReturnsCompleteRecords(t *testing.T) {
	t.Run("WithAttrs attributes are included", func(t *testing.T) {
		collector := NewLogCollector(nil)

		// Create a logger with collector attributes
		logger := slog.New(collector.WithAttrs([]slog.Attr{
			slog.String("service", "api"),
			slog.Int("version", 2),
		}))

		// Log a message with its own attributes
		logger.Info("request handled", "duration", 100, "status", 200)

		// Get the logs
		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}

		// Check that all attributes are present
		attrs := make(map[string]any)
		for _, attr := range logs[0].Attrs {
			attrs[attr.Key] = attr.Value.Any()
		}

		// Verify collector attributes
		if v, ok := attrs["service"]; !ok || v != "api" {
			t.Errorf("Missing or incorrect service attribute: %v", v)
		}
		if v, ok := attrs["version"]; !ok || v != int64(2) {
			t.Errorf("Missing or incorrect version attribute: %v", v)
		}

		// Verify message attributes
		if v, ok := attrs["duration"]; !ok || v != int64(100) {
			t.Errorf("Missing or incorrect duration attribute: %v", v)
		}
		if v, ok := attrs["status"]; !ok || v != int64(200) {
			t.Errorf("Missing or incorrect status attribute: %v", v)
		}
	})

	t.Run("Groups are properly applied", func(t *testing.T) {
		collector := NewLogCollector(nil)

		// Create a logger with groups and attributes
		logger := slog.New(
			collector.
				WithAttrs([]slog.Attr{slog.String("global", "value")}).
				WithGroup("server").
				WithAttrs([]slog.Attr{slog.String("host", "localhost")}),
		)

		// Log a message
		logger.Info("server started", "port", 8080)

		// Get the logs
		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}

		// Check attributes
		attrs := make(map[string]any)
		for _, attr := range logs[0].Attrs {
			flattenAttrs(attr, "", attrs)
		}

		// Global attribute should remain global
		if v, ok := attrs["global"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect global attribute: %v", v)
		}

		// Grouped attributes should be nested
		if v, ok := attrs["server.host"]; !ok || v != "localhost" {
			t.Errorf("Missing or incorrect server.host attribute: %v", v)
		}
		if v, ok := attrs["server.port"]; !ok || v != int64(8080) {
			t.Errorf("Missing or incorrect server.port attribute: %v", v)
		}
	})
}

// flattenAttrs recursively flattens nested attributes into a map with dotted keys
func flattenAttrs(attr slog.Attr, prefix string, result map[string]any) {
	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if attr.Value.Kind() == slog.KindGroup {
		// Recursively flatten group attributes
		for _, groupAttr := range attr.Value.Group() {
			flattenAttrs(groupAttr, key, result)
		}
	} else {
		result[key] = attr.Value.Any()
	}
}

// TestStorageReturnsRawRecords verifies that storage.GetAll() returns raw records
// without realization, which is needed for correct replay behavior
func TestStorageReturnsRawRecords(t *testing.T) {
	store := storage.NewRecordStorage()

	// Create a record with a sequence
	record := storage.Record{
		Time:    time.Now(),
		Level:   slog.LevelInfo,
		Message: "test",
		Attrs:   []slog.Attr{slog.String("msg_attr", "value")},
		Sequence: storage.HandlerSequence{
			{Type: "attrs", Attrs: []slog.Attr{slog.String("global", "value")}},
			{Type: "group", Group: "g1"},
			{Type: "attrs", Attrs: []slog.Attr{slog.String("grouped", "value")}},
		},
	}

	store.Append(&record)

	// Get records from storage
	records := store.GetAll()
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// Verify the raw record only has message attributes
	if len(records[0].Attrs) != 1 {
		t.Errorf("Expected 1 attribute in raw record, got %d", len(records[0].Attrs))
	}

	if records[0].Attrs[0].Key != "msg_attr" {
		t.Errorf("Expected msg_attr in raw record, got %s", records[0].Attrs[0].Key)
	}

	// Verify sequence is preserved
	if len(records[0].Sequence) != 3 {
		t.Errorf("Expected sequence of 3 operations, got %d", len(records[0].Sequence))
	}
}

// TestRecordRealize verifies that Record.Realize() correctly applies sequences
func TestRecordRealize(t *testing.T) {
	t.Run("Simple attributes", func(t *testing.T) {
		record := storage.Record{
			Time:    time.Now(),
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("msg", "value")},
			Sequence: storage.HandlerSequence{
				{Type: "attrs", Attrs: []slog.Attr{slog.String("collector", "value")}},
			},
		}

		realized := record.Realize()

		// Should have both collector and message attributes
		if len(realized.Attrs) != 2 {
			t.Fatalf("Expected 2 attributes, got %d", len(realized.Attrs))
		}

		// Collector attribute should come first
		if realized.Attrs[0].Key != "collector" {
			t.Errorf("Expected first attribute to be 'collector', got %s", realized.Attrs[0].Key)
		}

		// Message attribute should come second
		if realized.Attrs[1].Key != "msg" {
			t.Errorf("Expected second attribute to be 'msg', got %s", realized.Attrs[1].Key)
		}
	})

	t.Run("Groups and attributes", func(t *testing.T) {
		record := storage.Record{
			Time:    time.Now(),
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("msg", "value")},
			Sequence: storage.HandlerSequence{
				{Type: "attrs", Attrs: []slog.Attr{slog.String("global", "value")}},
				{Type: "group", Group: "g1"},
				{Type: "attrs", Attrs: []slog.Attr{slog.String("grouped", "value")}},
			},
		}

		realized := record.Realize()

		attrs := make(map[string]any)
		for _, attr := range realized.Attrs {
			flattenAttrs(attr, "", attrs)
		}

		// Global attribute should remain global
		if v, ok := attrs["global"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect global attribute: %v", v)
		}

		// Grouped attribute should be under g1
		if v, ok := attrs["g1.grouped"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect g1.grouped attribute: %v", v)
		}

		// Message attribute should be under g1 too
		if v, ok := attrs["g1.msg"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect g1.msg attribute: %v", v)
		}
	})
}
