package storage

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func TestNewRecord(t *testing.T) {
	// Create fixed time for testing
	fixedTime := time.Date(2025, 5, 14, 12, 0, 0, 0, time.UTC)

	t.Run("Valid record creation", func(t *testing.T) {
		// Create a slog.Record with attributes
		slogRecord := slog.NewRecord(fixedTime, slog.LevelError, "test message", 0)
		slogRecord.AddAttrs(
			slog.String("key1", "value1"),
			slog.Int("key2", 42),
			slog.Bool("key3", true),
		)

		// Create a storage.Record
		record := NewRecord(context.Background(), nil, &slogRecord)

		// Basic validation
		if record == nil {
			t.Fatal("Expected non-nil record")
		}

		// Validate fields
		if !record.Time.Equal(fixedTime) {
			t.Errorf("Expected time %v, got %v", fixedTime, record.Time)
		}

		if record.Level != slog.LevelError {
			t.Errorf("Expected level ERROR, got %v", record.Level)
		}

		if record.Message != "test message" {
			t.Errorf("Expected message 'test message', got %q", record.Message)
		}

		// Validate all attributes were copied
		if len(record.Attrs) != 3 {
			t.Errorf("Expected 3 attributes, got %d", len(record.Attrs))
		}

		// Verify attribute values by looping and checking each
		expectedAttrs := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		}

		for _, attr := range record.Attrs {
			expectedValue, exists := expectedAttrs[attr.Key]
			if !exists {
				t.Errorf("Unexpected attribute: %s", attr.Key)
				continue
			}

			// Convert slog.Value to comparable Go type
			var actualValue interface{}
			switch attr.Value.Kind() {
			case slog.KindString:
				actualValue = attr.Value.String()
			case slog.KindInt64:
				actualValue = int(attr.Value.Int64())
			case slog.KindBool:
				actualValue = attr.Value.Bool()
			default:
				t.Errorf("Unexpected value kind: %v", attr.Value.Kind())
			}

			if !reflect.DeepEqual(expectedValue, actualValue) {
				t.Errorf("Attribute %s: expected %v, got %v", attr.Key, expectedValue, actualValue)
			}
		}
	})

	t.Run("Nil record input", func(t *testing.T) {
		// Test with nil slog.Record
		record := NewRecord(context.Background(), nil, nil)
		if record != nil {
			t.Error("Expected nil record when input is nil")
		}
	})

	t.Run("Empty groups", func(t *testing.T) {
		// Create record with no groups
		slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "no groups", 0)
		record := NewRecord(context.Background(), nil, &slogRecord)

		if record == nil {
			t.Fatal("Expected non-nil record")
		}

	})

	t.Run("No attributes", func(t *testing.T) {
		// Create record with no attributes
		slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "no attrs", 0)
		record := NewRecord(context.Background(), nil, &slogRecord)

		if record == nil {
			t.Fatal("Expected non-nil record")
		}

		// Verify attrs is empty but initialized
		if record.Attrs == nil {
			t.Error("Expected non-nil Attrs slice")
		}

		if len(record.Attrs) != 0 {
			t.Errorf("Expected empty attributes, got %v", record.Attrs)
		}
	})

	t.Run("Record with mixed attribute types", func(t *testing.T) {
		// Create a slog record with various attribute types
		slogRecord := slog.NewRecord(fixedTime, slog.LevelWarn, "mixed attrs", 0)
		slogRecord.AddAttrs(
			slog.String("string", "hello"),
			slog.Int("int", 42),
			slog.Float64("float", 3.14),
			slog.Bool("bool", true),
			slog.Time("time", fixedTime),
			slog.Duration("duration", 5*time.Second),
			slog.Group("group",
				slog.String("nested", "value"),
				slog.Int("count", 1),
			),
		)

		// Create storage record
		record := NewRecord(context.Background(), nil, &slogRecord)

		if record == nil {
			t.Fatal("Expected non-nil record")
		}

		// Verify all attributes were copied
		// Note: groups become flattened, so we expect more attributes
		if len(record.Attrs) < 7 {
			t.Errorf("Expected at least 7 attributes, got %d", len(record.Attrs))
		}

		// Log the attributes for debugging
		t.Logf("Attributes (%d):", len(record.Attrs))
		for i, attr := range record.Attrs {
			t.Logf("  attr[%d]: %s=%v", i, attr.Key, attr.Value)
		}
	})

	t.Run("PC preservation", func(t *testing.T) {
		slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "pc test", 12345)
		record := NewRecord(context.Background(), nil, &slogRecord)

		if record == nil {
			t.Fatal("Expected non-nil record")
		}

		if record.PC != 12345 {
			t.Errorf("Expected PC 12345, got %d", record.PC)
		}
	})

	t.Run("Journal preservation", func(t *testing.T) {
		journal := OperationJournal{
			{Type: OpAttrs, Attrs: []slog.Attr{slog.String("global", "value")}},
			{Type: OpGroup, Group: "api"},
		}

		slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "journal test", 0)
		record := NewRecord(context.Background(), journal, &slogRecord)

		if record == nil {
			t.Fatal("Expected non-nil record")
		}

		if len(record.Journal) != 2 {
			t.Errorf("Expected journal length 2, got %d", len(record.Journal))
		}

		if record.Journal[0].Type != OpAttrs {
			t.Errorf("Expected first operation type OpAttrs, got %v", record.Journal[0].Type)
		}

		if record.Journal[1].Type != OpGroup {
			t.Errorf("Expected second operation type OpGroup, got %v", record.Journal[1].Type)
		}

		if record.Journal[1].Group != "api" {
			t.Errorf("Expected group name 'api', got %q", record.Journal[1].Group)
		}
	})
}

func TestRecordRealize(t *testing.T) {
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	t.Run("AppliesJournalCorrectly", func(t *testing.T) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			PC:      123,
			Attrs:   []slog.Attr{slog.String("msg", "value")},
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("global", "value")}},
				{Type: OpGroup, Group: "api"},
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("user", "123")}},
			},
		}

		realized := record.Realize()

		if realized.Time != fixedTime {
			t.Errorf("Time not preserved")
		}
		if realized.Level != slog.LevelInfo {
			t.Errorf("Level not preserved")
		}
		if realized.Message != "test" {
			t.Errorf("Message not preserved")
		}
		if realized.PC != 123 {
			t.Errorf("PC not preserved")
		}

		if len(realized.Attrs) != 3 {
			t.Fatalf("Expected 3 attributes, got %d", len(realized.Attrs))
		}

		if realized.Attrs[0].Key != "global" {
			t.Errorf("Expected first attribute to be 'global', got %q", realized.Attrs[0].Key)
		}
	})

	t.Run("HandlesEmptyJournal", func(t *testing.T) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("key", "value")},
			Journal: OperationJournal{},
		}

		realized := record.Realize()

		if len(realized.Attrs) != 1 {
			t.Fatalf("Expected 1 attribute, got %d", len(realized.Attrs))
		}

		if realized.Attrs[0].Key != "key" {
			t.Errorf("Expected attribute key 'key', got %q", realized.Attrs[0].Key)
		}
	})

	t.Run("PreservesOriginalRecord", func(t *testing.T) {
		original := Record{
			Time:    fixedTime,
			Level:   slog.LevelError,
			Message: "original message",
			PC:      123,
			Attrs:   []slog.Attr{slog.String("original", "attr")},
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("added", "attr")}},
			},
		}

		realized := original.Realize()

		if original.Message != "original message" {
			t.Errorf("Original message changed")
		}

		if len(original.Attrs) != 1 {
			t.Errorf("Original attrs changed")
		}

		if len(realized.Attrs) != 2 {
			t.Errorf("Expected 2 realized attributes, got %d", len(realized.Attrs))
		}
	})

	t.Run("IgnoresUnknownOperationType", func(t *testing.T) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("msg", "value")},
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("global", "value")}},
				{Type: OperationType(999), Group: "invalid"}, // Unknown operation type
				{Type: OpGroup, Group: "valid"},
			},
		}

		realized := record.Realize()

		// Should have processed global attr and valid group, ignored unknown op
		if len(realized.Attrs) != 2 {
			t.Errorf("Expected 2 attributes (global + grouped msg), got %d", len(realized.Attrs))
		}

		attrs := make(map[string]any)
		for _, attr := range realized.Attrs {
			flattenAttrs(attr, "", attrs)
		}

		if v, ok := attrs["global"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect global attribute: %v", v)
		}
		if v, ok := attrs["valid.msg"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect valid.msg attribute: %v", v)
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
		for _, groupAttr := range attr.Value.Group() {
			flattenAttrs(groupAttr, key, result)
		}
	} else {
		result[key] = attr.Value.Any()
	}
}

func TestApplyGroups(t *testing.T) {
	t.Run("NoGroups", func(t *testing.T) {
		attr := slog.String("key", "value")
		result := applyGroups(attr, nil)

		if result.Key != "key" {
			t.Errorf("Expected key 'key', got %q", result.Key)
		}
	})

	t.Run("EmptyGroups", func(t *testing.T) {
		attr := slog.String("key", "value")
		result := applyGroups(attr, []string{})

		if result.Key != "key" {
			t.Errorf("Expected key 'key', got %q", result.Key)
		}
	})

	t.Run("SingleGroup", func(t *testing.T) {
		attr := slog.String("key", "value")
		result := applyGroups(attr, []string{"group1"})

		if result.Key != "group1" {
			t.Errorf("Expected key 'group1', got %q", result.Key)
		}

		if result.Value.Kind() != slog.KindGroup {
			t.Errorf("Expected group value, got %v", result.Value.Kind())
		}
	})

	t.Run("NestedGroups", func(t *testing.T) {
		attr := slog.String("key", "value")
		result := applyGroups(attr, []string{"outer", "inner"})

		if result.Key != "outer" {
			t.Errorf("Expected key 'outer', got %q", result.Key)
		}

		if result.Value.Kind() != slog.KindGroup {
			t.Errorf("Expected group value, got %v", result.Value.Kind())
		}

		outerGroup := result.Value.Group()
		if len(outerGroup) != 1 {
			t.Fatalf("Expected 1 item in outer group, got %d", len(outerGroup))
		}

		if outerGroup[0].Key != "inner" {
			t.Errorf("Expected inner key 'inner', got %q", outerGroup[0].Key)
		}
	})
}
