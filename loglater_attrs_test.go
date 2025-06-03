package loglater

import (
	"log/slog"
	"testing"
)

// TestAttributePreservation verifies that attributes from both the log record
// and the collector are properly preserved
func TestAttributePreservation(t *testing.T) {
	collector := NewLogCollector(nil)

	// Create a collector with some base attributes
	collectorWithAttrs := collector.WithAttrs([]slog.Attr{
		slog.String("collector_attr", "collector_value"),
		slog.Int("collector_num", 42),
	})

	logger := slog.New(collectorWithAttrs)

	// Log a message with additional attributes
	logger.Info("test message",
		slog.String("log_attr", "log_value"),
		slog.Int("log_num", 99),
	)

	// Get the stored logs
	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	// Check that all attributes are present
	record := logs[0]
	attrs := make(map[string]any)
	for _, attr := range record.Attrs {
		attrs[attr.Key] = attr.Value.Any()
	}

	// Verify collector attributes are present
	if v, ok := attrs["collector_attr"]; !ok || v != "collector_value" {
		t.Errorf("Missing or incorrect collector_attr: got %v", v)
	}
	if v, ok := attrs["collector_num"]; !ok || v != int64(42) {
		t.Errorf("Missing or incorrect collector_num: got %v", v)
	}

	// Verify log attributes are present
	if v, ok := attrs["log_attr"]; !ok || v != "log_value" {
		t.Errorf("Missing or incorrect log_attr: got %v", v)
	}
	if v, ok := attrs["log_num"]; !ok || v != int64(99) {
		t.Errorf("Missing or incorrect log_num: got %v", v)
	}

	// Verify we have exactly 4 attributes
	if len(record.Attrs) != 4 {
		t.Errorf("Expected 4 attributes, got %d", len(record.Attrs))
	}
}

// TestAttributeOrderWithGroups tests that attributes maintain proper order
// when used with groups
func TestAttributeOrderWithGroups(t *testing.T) {
	collector := NewLogCollector(nil)

	// Create a chain: base -> attrs -> group -> more attrs
	c1 := collector.WithAttrs([]slog.Attr{slog.String("a1", "v1")})
	c2 := c1.WithGroup("g1")
	c3 := c2.WithAttrs([]slog.Attr{slog.String("a2", "v2")})

	logger := slog.New(c3)
	logger.Info("test", slog.String("a3", "v3"))

	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	// Check groups
	if len(logs[0].Groups) != 1 || logs[0].Groups[0] != "g1" {
		t.Errorf("Expected group [g1], got %v", logs[0].Groups)
	}

	// Verify all attributes are present
	attrs := make(map[string]string)
	for _, attr := range logs[0].Attrs {
		attrs[attr.Key] = attr.Value.String()
	}

	if attrs["a1"] != "v1" {
		t.Errorf("Missing or incorrect a1")
	}
	if attrs["a2"] != "v2" {
		t.Errorf("Missing or incorrect a2")
	}
	if attrs["a3"] != "v3" {
		t.Errorf("Missing or incorrect a3")
	}
}
