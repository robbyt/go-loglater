package storage

import (
	"context"
	"fmt"
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

	t.Run("ComplexNestedGroupsWithMultipleAttrs", func(t *testing.T) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "complex test",
			Attrs: []slog.Attr{
				slog.String("request_id", "123"),
				slog.Int("status", 200),
			},
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{
					slog.String("service", "api"),
					slog.String("version", "v1"),
				}},
				{Type: OpGroup, Group: "http"},
				{Type: OpAttrs, Attrs: []slog.Attr{
					slog.String("method", "GET"),
					slog.String("path", "/users"),
				}},
				{Type: OpGroup, Group: "response"},
				{Type: OpAttrs, Attrs: []slog.Attr{
					slog.Duration("latency", 100*time.Millisecond),
				}},
			},
		}

		realized := record.Realize()

		// Should have:
		// - 2 global attrs (service, version)
		// - 2 attrs in http group (method, path)
		// - 1 attr in http.response group (latency)
		// - 2 record attrs in http.response group (request_id, status)
		// Total: 7 attributes
		if len(realized.Attrs) != 7 {
			t.Errorf("Expected 7 attributes, got %d", len(realized.Attrs))
		}

		// Flatten and verify structure
		attrs := make(map[string]any)
		for _, attr := range realized.Attrs {
			flattenAttrs(attr, "", attrs)
		}

		// Check global attributes
		if v, ok := attrs["service"]; !ok || v != "api" {
			t.Errorf("Missing or incorrect service attribute: %v", v)
		}
		if v, ok := attrs["version"]; !ok || v != "v1" {
			t.Errorf("Missing or incorrect version attribute: %v", v)
		}

		// Check http group attributes
		if v, ok := attrs["http.method"]; !ok || v != "GET" {
			t.Errorf("Missing or incorrect http.method attribute: %v", v)
		}
		if v, ok := attrs["http.path"]; !ok || v != "/users" {
			t.Errorf("Missing or incorrect http.path attribute: %v", v)
		}

		// Check nested http.response group attributes
		if _, ok := attrs["http.response.latency"]; !ok {
			t.Errorf("Missing http.response.latency attribute")
		}
		if v, ok := attrs["http.response.request_id"]; !ok || v != "123" {
			t.Errorf("Missing or incorrect http.response.request_id attribute: %v", v)
		}
		if v, ok := attrs["http.response.status"]; !ok || v != int64(200) {
			t.Errorf("Missing or incorrect http.response.status attribute: %v", v)
		}
	})

	t.Run("EmptyGroupNames", func(t *testing.T) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("key", "value")},
			Journal: OperationJournal{
				{Type: OpGroup, Group: ""}, // Empty group name
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("attr", "val")}},
			},
		}

		realized := record.Realize()

		// Empty group should still be applied
		if len(realized.Attrs) != 2 {
			t.Errorf("Expected 2 attributes, got %d", len(realized.Attrs))
		}

		// Both attrs should be within empty group
		for _, attr := range realized.Attrs {
			if attr.Key != "" {
				t.Errorf("Expected empty group key, got %q", attr.Key)
			}
			if attr.Value.Kind() != slog.KindGroup {
				t.Errorf("Expected group kind, got %v", attr.Value.Kind())
			}
		}
	})

	t.Run("RecordWithNoAttrs", func(t *testing.T) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "no attrs",
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("collector", "attr")}},
				{Type: OpGroup, Group: "group"},
			},
		}

		realized := record.Realize()

		// Should only have the collector attribute
		if len(realized.Attrs) != 1 {
			t.Errorf("Expected 1 attribute, got %d", len(realized.Attrs))
		}

		if realized.Attrs[0].Key != "collector" {
			t.Errorf("Expected attribute key 'collector', got %q", realized.Attrs[0].Key)
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

	t.Run("DeeplyNestedGroups", func(t *testing.T) {
		attr := slog.Int("count", 42)
		result := applyGroups(attr, []string{"level1", "level2", "level3", "level4"})

		// Verify top level
		if result.Key != "level1" {
			t.Errorf("Expected key 'level1', got %q", result.Key)
		}

		// Navigate through all levels
		current := result
		for i, expectedKey := range []string{"level1", "level2", "level3", "level4"} {
			if current.Key != expectedKey {
				t.Errorf("Level %d: expected key %q, got %q", i, expectedKey, current.Key)
			}

			if i < 3 { // Not the last level
				if current.Value.Kind() != slog.KindGroup {
					t.Errorf("Level %d: expected group kind, got %v", i, current.Value.Kind())
				}
				group := current.Value.Group()
				if len(group) != 1 {
					t.Fatalf("Level %d: expected 1 item in group, got %d", i, len(group))
				}
				current = group[0]
			} else { // Last level should contain the actual attribute
				group := current.Value.Group()
				if len(group) != 1 {
					t.Fatalf("Final level: expected 1 item in group, got %d", len(group))
				}
				if group[0].Key != "count" {
					t.Errorf("Expected final attribute key 'count', got %q", group[0].Key)
				}
				if group[0].Value.Int64() != 42 {
					t.Errorf("Expected final value 42, got %d", group[0].Value.Int64())
				}
			}
		}
	})
}

func BenchmarkNewRecord(b *testing.B) {
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	b.Run("WithoutAttrs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "test message", 0)
			_ = NewRecord(ctx, nil, &slogRecord)
		}
	})

	b.Run("With3Attrs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "test message", 0)
			slogRecord.AddAttrs(
				slog.String("key1", "value1"),
				slog.Int("key2", 42),
				slog.Bool("key3", true),
			)
			_ = NewRecord(ctx, nil, &slogRecord)
		}
	})

	b.Run("With10Attrs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "test message", 0)
			for j := 0; j < 10; j++ {
				slogRecord.AddAttrs(slog.String(fmt.Sprintf("key%d", j), fmt.Sprintf("value%d", j)))
			}
			_ = NewRecord(ctx, nil, &slogRecord)
		}
	})

	b.Run("WithJournal", func(b *testing.B) {
		journal := OperationJournal{
			{Type: OpAttrs, Attrs: []slog.Attr{slog.String("global", "value")}},
			{Type: OpGroup, Group: "api"},
			{Type: OpAttrs, Attrs: []slog.Attr{slog.String("method", "GET")}},
		}
		b.ReportAllocs()
		for b.Loop() {
			slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "test message", 0)
			slogRecord.AddAttrs(slog.String("key", "value"))
			_ = NewRecord(ctx, journal, &slogRecord)
		}
	})
}

func BenchmarkRecordRealize(b *testing.B) {
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	b.Run("NoJournal", func(b *testing.B) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("key", "value")},
			Journal: OperationJournal{},
		}
		b.ReportAllocs()
		for b.Loop() {
			_ = record.Realize()
		}
	})

	b.Run("SimpleJournal", func(b *testing.B) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("msg", "value")},
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{slog.String("global", "value")}},
				{Type: OpGroup, Group: "api"},
			},
		}
		b.ReportAllocs()
		for b.Loop() {
			_ = record.Realize()
		}
	})

	b.Run("ComplexJournal", func(b *testing.B) {
		record := Record{
			Time:    fixedTime,
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs: []slog.Attr{
				slog.String("request_id", "123"),
				slog.Int("status", 200),
			},
			Journal: OperationJournal{
				{Type: OpAttrs, Attrs: []slog.Attr{
					slog.String("service", "api"),
					slog.String("version", "v1"),
				}},
				{Type: OpGroup, Group: "http"},
				{Type: OpAttrs, Attrs: []slog.Attr{
					slog.String("method", "GET"),
					slog.String("path", "/users"),
				}},
				{Type: OpGroup, Group: "response"},
				{Type: OpAttrs, Attrs: []slog.Attr{
					slog.Duration("latency", 100*time.Millisecond),
				}},
			},
		}
		b.ReportAllocs()
		for b.Loop() {
			_ = record.Realize()
		}
	})
}

func BenchmarkApplyGroups(b *testing.B) {
	attr := slog.String("key", "value")

	b.Run("NoGroups", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = applyGroups(attr, nil)
		}
	})

	b.Run("SingleGroup", func(b *testing.B) {
		groups := []string{"group1"}
		b.ReportAllocs()
		for b.Loop() {
			_ = applyGroups(attr, groups)
		}
	})

	b.Run("ThreeGroups", func(b *testing.B) {
		groups := []string{"level1", "level2", "level3"}
		b.ReportAllocs()
		for b.Loop() {
			_ = applyGroups(attr, groups)
		}
	})

	b.Run("FiveGroups", func(b *testing.B) {
		groups := []string{"level1", "level2", "level3", "level4", "level5"}
		b.ReportAllocs()
		for b.Loop() {
			_ = applyGroups(attr, groups)
		}
	})
}
