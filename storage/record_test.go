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

		// Create groups
		groups := []string{"group1", "group2"}

		// Create a storage.Record
		record := NewRecord(context.Background(), groups, &slogRecord)

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

		// Validate groups
		if !reflect.DeepEqual(record.Groups, groups) {
			t.Errorf("Expected groups %v, got %v", groups, record.Groups)
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
		record := NewRecord(context.Background(), []string{"group"}, nil)
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

		// Verify groups is nil or empty
		if len(record.Groups) != 0 {
			t.Errorf("Expected empty groups, got %v", record.Groups)
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

	t.Run("Context propagation", func(t *testing.T) {
		// Test that context is correctly used (currently the implementation doesn't use context,
		// but we should test for it in case that changes in the future)

		// Create a context with a value
		type ctxKey string
		testKey := ctxKey("test-key")
		testValue := "test-value"
		ctx := context.WithValue(context.Background(), testKey, testValue)

		// Create a record
		slogRecord := slog.NewRecord(fixedTime, slog.LevelInfo, "context test", 0)
		record := NewRecord(ctx, nil, &slogRecord)

		if record == nil {
			t.Fatal("Expected non-nil record")
		}

		// Basic checks to ensure record was created correctly
		if record.Message != "context test" {
			t.Errorf("Expected message 'context test', got %q", record.Message)
		}

		// The current implementation doesn't use context, but this test ensures
		// we have coverage if that changes in the future
	})
}
