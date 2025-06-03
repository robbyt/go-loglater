package loglater

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robbyt/go-loglater/storage"
)

// Interface compliance
var _ slog.Handler = (*LogCollector)(nil)

func TestLogCollectorImplementsSlogHandler(t *testing.T) {
	var handler slog.Handler = NewLogCollector(nil)
	_ = handler.Enabled(context.Background(), slog.LevelInfo)
	_ = handler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	_ = handler.WithGroup("groupname")
	_ = handler.Handle(context.Background(), slog.Record{})
}

// Core functionality
func TestBasicLogging(t *testing.T) {
	t.Run("CollectAndReplay", func(t *testing.T) {
		var buf bytes.Buffer
		collector := NewLogCollector(nil)
		logger := slog.New(collector)

		logger.WithGroup("app").Info("Starting up", "version", "1.0")
		logger.Error("Something failed", "error", "test error")

		logs := collector.GetLogs()
		if len(logs) != 2 {
			t.Fatalf("expected 2 logs, got %d", len(logs))
		}

		if logs[0].Message != "Starting up" {
			t.Errorf("expected 'Starting up', got %q", logs[0].Message)
		}

		if logs[1].Level != slog.LevelError {
			t.Errorf("expected Error level, got %v", logs[1].Level)
		}

		jsonHandler := slog.NewJSONHandler(&buf, nil)
		if err := collector.PlayLogs(jsonHandler); err != nil {
			t.Fatalf("unexpected error playing logs: %v", err)
		}

		if buf.Len() == 0 {
			t.Error("expected non-empty buffer after replay")
		}
	})

	t.Run("ForwardToHandler", func(t *testing.T) {
		var buf bytes.Buffer
		jsonHandler := slog.NewJSONHandler(&buf, nil)
		collector := NewLogCollector(jsonHandler)
		logger := slog.New(collector)

		logger.Info("This should be forwarded immediately")

		if buf.Len() == 0 {
			t.Error("expected output to be forwarded immediately")
		}

		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("expected 1 collected log, got %d", len(logs))
		}
	})

	t.Run("EnabledLevels", func(t *testing.T) {
		opts := &slog.HandlerOptions{Level: slog.LevelInfo}
		var buf bytes.Buffer
		jsonHandler := slog.NewJSONHandler(&buf, opts)
		collector := NewLogCollector(jsonHandler)

		if collector.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("expected DEBUG level to be disabled")
		}
		if !collector.Enabled(context.Background(), slog.LevelInfo) {
			t.Error("expected INFO level to be enabled")
		}

		nilCollector := NewLogCollector(nil)
		if !nilCollector.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("expected all levels to be enabled with nil handler")
		}
	})
}

// Attributes and groups - the critical test that was flaky
func TestAttributesAndGroups(t *testing.T) {
	t.Run("WithGroupAndAttributes", func(t *testing.T) {
		var origBuf, replayBuf bytes.Buffer

		textHandler := slog.NewTextHandler(&origBuf, nil)
		collector := NewLogCollector(textHandler)
		logger := slog.New(collector)

		baseLogger := logger.With("global", "value")
		groupLogger := baseLogger.WithGroup("group1")
		groupWithAttrLogger := groupLogger.With("attribute", "value")
		nestedGroupLogger := logger.WithGroup("parent").WithGroup("child")
		multiAttrLogger := logger.WithGroup("multi").With("attr1", "value1", "attr2", "value2")

		baseLogger.Info("Base log")
		groupLogger.Info("Group log", "field1", "value1")
		groupWithAttrLogger.Error("Group with attr log", "field2", "value2")
		nestedGroupLogger.Warn("Nested group log", "nested", "value")
		multiAttrLogger.Info("Multiple attrs", "extra", "value")

		origOutput := origBuf.String()
		origLogs := parseLogOutput(t, origOutput)

		replayHandler := slog.NewTextHandler(&replayBuf, nil)
		if err := collector.PlayLogs(replayHandler); err != nil {
			t.Fatalf("Failed to replay logs: %v", err)
		}

		replayOutput := replayBuf.String()
		replayLogs := parseLogOutput(t, replayOutput)

		if len(origLogs) != len(replayLogs) {
			t.Errorf("Different number of log lines: original=%d, replayed=%d",
				len(origLogs), len(replayLogs))
		}

		for timestamp, origFields := range origLogs {
			replayFields, found := replayLogs[timestamp]
			if !found {
				t.Errorf("Log entry with timestamp %s missing in replay", timestamp)
				continue
			}
			compareLogFields(t, timestamp, origFields, replayFields)
		}
	})

	t.Run("GetLogsReturnsCompleteRecords", func(t *testing.T) {
		collector := NewLogCollector(nil)

		logger := slog.New(collector.WithAttrs([]slog.Attr{
			slog.String("service", "api"),
			slog.Int("version", 2),
		}))

		logger.Info("request handled", "duration", 100, "status", 200)

		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}

		attrs := make(map[string]any)
		for _, attr := range logs[0].Attrs {
			attrs[attr.Key] = attr.Value.Any()
		}

		if v, ok := attrs["service"]; !ok || v != "api" {
			t.Errorf("Missing or incorrect service attribute: %v", v)
		}
		if v, ok := attrs["version"]; !ok || v != int64(2) {
			t.Errorf("Missing or incorrect version attribute: %v", v)
		}
		if v, ok := attrs["duration"]; !ok || v != int64(100) {
			t.Errorf("Missing or incorrect duration attribute: %v", v)
		}
		if v, ok := attrs["status"]; !ok || v != int64(200) {
			t.Errorf("Missing or incorrect status attribute: %v", v)
		}
	})

	t.Run("GroupsProperlyApplied", func(t *testing.T) {
		collector := NewLogCollector(nil)

		logger := slog.New(
			collector.
				WithAttrs([]slog.Attr{slog.String("global", "value")}).
				WithGroup("server").
				WithAttrs([]slog.Attr{slog.String("host", "localhost")}),
		)

		logger.Info("server started", "port", 8080)

		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}

		attrs := make(map[string]any)
		for _, attr := range logs[0].Attrs {
			flattenAttrs(attr, "", attrs)
		}

		if v, ok := attrs["global"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect global attribute: %v", v)
		}
		if v, ok := attrs["server.host"]; !ok || v != "localhost" {
			t.Errorf("Missing or incorrect server.host attribute: %v", v)
		}
		if v, ok := attrs["server.port"]; !ok || v != int64(8080) {
			t.Errorf("Missing or incorrect server.port attribute: %v", v)
		}
	})
}

// Error handling and edge cases
func TestErrorHandling(t *testing.T) {
	t.Run("WithAttrsEmptySlice", func(t *testing.T) {
		collector := NewLogCollector(nil)
		newHandler := collector.WithAttrs([]slog.Attr{})
		if newHandler != collector {
			t.Error("Expected WithAttrs with empty slice to return same handler")
		}
	})

	t.Run("WithGroupEmptyName", func(t *testing.T) {
		collector := NewLogCollector(nil)
		newHandler := collector.WithGroup("")
		if newHandler != collector {
			t.Error("Expected WithGroup with empty name to return same handler")
		}
	})

	t.Run("PlayLogsNilHandler", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)
		logger.Info("Test message")

		err := collector.PlayLogs(nil)
		if err == nil || !strings.Contains(err.Error(), "handler is nil") {
			t.Errorf("Expected error for nil handler, got: %v", err)
		}
	})

	t.Run("PlayLogsErrorHandler", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)
		logger.Info("Test message")

		errHandler := &errorHandler{}
		err := collector.PlayLogs(errHandler)

		if err != io.ErrUnexpectedEOF {
			t.Errorf("Expected PlayLogs to return io.ErrUnexpectedEOF, got %v", err)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)

		for i := 0; i < 5; i++ {
			logger.Info("Test message", "index", i)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := collector.PlayLogsCtx(ctx, slog.NewTextHandler(io.Discard, nil))
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got: %v", err)
		}
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)
		logger.Info("Test message")

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		slowHandler := &sleepyHandler{sleepTime: 100 * time.Millisecond}
		err := collector.PlayLogsCtx(ctx, slowHandler)
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded error, got: %v", err)
		}
	})

	t.Run("GetLogsCopiesData", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)
		logger.Info("test message")

		logs1 := collector.GetLogs()
		if len(logs1) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs1))
		}

		logs2 := collector.GetLogs()
		logger.Info("second message")

		if len(logs1) != 1 || len(logs2) != 1 {
			t.Error("Expected previous GetLogs results to be unchanged")
		}

		logs3 := collector.GetLogs()
		if len(logs3) != 2 {
			t.Errorf("Expected final GetLogs to have 2 logs, got %d", len(logs3))
		}
	})
}

// Concurrency tests
func TestConcurrency(t *testing.T) {
	t.Run("ConcurrentLoggingWithSharedStorage", func(t *testing.T) {
		collector := NewLogCollector(nil)

		collector1 := collector.WithAttrs([]slog.Attr{slog.String("collector", "1")})
		collector2 := collector.WithAttrs([]slog.Attr{slog.String("collector", "2")})
		collector3 := collector.WithGroup("group1")

		logger1 := slog.New(collector1)
		logger2 := slog.New(collector2)
		logger3 := slog.New(collector3)

		var wg sync.WaitGroup
		iterations := 100
		wg.Add(3)

		go func() {
			defer wg.Done()
			for i := range iterations {
				logger1.Info("test message", "index", i, "logger", 1)
			}
		}()

		go func() {
			defer wg.Done()
			for i := range iterations {
				logger2.Info("test message", "index", i, "logger", 2)
			}
		}()

		go func() {
			defer wg.Done()
			for i := range iterations {
				logger3.Info("test message", "index", i, "logger", 3)
			}
		}()

		wg.Wait()

		logs := collector.GetLogs()
		expectedCount := iterations * 3
		if len(logs) != expectedCount {
			t.Errorf("Expected %d logs, got %d", expectedCount, len(logs))
		}
	})

	t.Run("ConcurrentPlayLogs", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)

		for i := range 100 {
			logger.Info("test message", "index", i)
		}

		var wg sync.WaitGroup
		wg.Add(3)

		for range 3 {
			go func() {
				defer wg.Done()
				handler := slog.NewTextHandler(&discardWriter{}, nil)
				err := collector.PlayLogs(handler)
				if err != nil {
					t.Errorf("PlayLogs failed: %v", err)
				}
			}()
		}

		wg.Wait()
	})
}

// PC preservation tests
func TestPCPreservation(t *testing.T) {
	t.Run("BasicPCPreservation", func(t *testing.T) {
		collector := NewLogCollector(nil)
		logger := slog.New(collector)

		logger.Info("test message with PC")

		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}

		if logs[0].PC == 0 {
			t.Error("Expected non-zero PC value")
		}

		var buf bytes.Buffer
		handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{AddSource: true})

		err := collector.PlayLogs(handler)
		if err != nil {
			t.Fatalf("PlayLogs failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "loglater_test.go") {
			t.Errorf("Expected source file in output, got: %s", output)
		}
	})

	t.Run("PCWithGroupsAndAttrs", func(t *testing.T) {
		collector := NewLogCollector(nil)

		logger := slog.New(collector.WithGroup("testgroup").WithAttrs([]slog.Attr{
			slog.String("component", "test"),
		}))

		logger.Info("grouped message with attrs")

		logs := collector.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("Expected 1 log, got %d", len(logs))
		}

		if logs[0].PC == 0 {
			t.Error("Expected non-zero PC value with groups and attrs")
		}
	})
}

// Storage behavior and consistency tests
func TestStorageBehavior(t *testing.T) {
	t.Run("StorageReturnsRawRecords", func(t *testing.T) {
		store := storage.NewRecordStorage()

		record := storage.Record{
			Time:    time.Now(),
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("msg_attr", "value")},
			Sequence: storage.OperationJournal{
				{Type: "attrs", Attrs: []slog.Attr{slog.String("global", "value")}},
				{Type: "group", Group: "g1"},
				{Type: "attrs", Attrs: []slog.Attr{slog.String("grouped", "value")}},
			},
		}

		store.Append(&record)

		records := store.GetAll()
		if len(records) != 1 {
			t.Fatalf("Expected 1 record, got %d", len(records))
		}

		if len(records[0].Attrs) != 1 {
			t.Errorf("Expected 1 attribute in raw record, got %d", len(records[0].Attrs))
		}

		if records[0].Attrs[0].Key != "msg_attr" {
			t.Errorf("Expected msg_attr in raw record, got %s", records[0].Attrs[0].Key)
		}

		if len(records[0].Sequence) != 3 {
			t.Errorf("Expected sequence of 3 operations, got %d", len(records[0].Sequence))
		}
	})

	t.Run("RecordRealize", func(t *testing.T) {
		record := storage.Record{
			Time:    time.Now(),
			Level:   slog.LevelInfo,
			Message: "test",
			Attrs:   []slog.Attr{slog.String("msg", "value")},
			Sequence: storage.OperationJournal{
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

		if v, ok := attrs["global"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect global attribute: %v", v)
		}
		if v, ok := attrs["g1.grouped"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect g1.grouped attribute: %v", v)
		}
		if v, ok := attrs["g1.msg"]; !ok || v != "value" {
			t.Errorf("Missing or incorrect g1.msg attribute: %v", v)
		}
	})

	t.Run("GetLogsPlayLogsConsistency", func(t *testing.T) {
		collector := NewLogCollector(nil)

		logger := slog.New(
			collector.
				WithAttrs([]slog.Attr{slog.String("service", "api")}).
				WithGroup("request").
				WithAttrs([]slog.Attr{slog.String("id", "123")}),
		)

		logger.Info("start", "method", "GET")
		logger.WithGroup("response").Info("complete", "status", 200)

		logs := collector.GetLogs()
		if len(logs) != 2 {
			t.Fatalf("Expected 2 logs, got %d", len(logs))
		}

		attrs1 := make(map[string]any)
		for _, attr := range logs[0].Attrs {
			flattenAttrs(attr, "", attrs1)
		}

		if v, ok := attrs1["service"]; !ok || v != "api" {
			t.Errorf("First log missing service=api: %v", attrs1)
		}
		if v, ok := attrs1["request.id"]; !ok || v != "123" {
			t.Errorf("First log missing request.id=123: %v", attrs1)
		}
		if v, ok := attrs1["request.method"]; !ok || v != "GET" {
			t.Errorf("First log missing request.method=GET: %v", attrs1)
		}

		var buf bytes.Buffer
		textHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "time" {
					return slog.Attr{}
				}
				return a
			},
		})

		if err := collector.PlayLogs(textHandler); err != nil {
			t.Fatalf("PlayLogs failed: %v", err)
		}

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) != 2 {
			t.Fatalf("Expected 2 log lines, got %d: %s", len(lines), output)
		}

		line1 := lines[0]
		if !strings.Contains(line1, "service=api") {
			t.Errorf("First line missing service=api: %s", line1)
		}
		if !strings.Contains(line1, "request.id=123") {
			t.Errorf("First line missing request.id=123: %s", line1)
		}
		if !strings.Contains(line1, "request.method=GET") {
			t.Errorf("First line missing request.method=GET: %s", line1)
		}
	})
}

// Helper functions
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

func parseLogLine(t *testing.T, line string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	var fields []string
	var currentField strings.Builder
	inQuote := false

	for _, r := range line {
		if r == '"' {
			inQuote = !inQuote
			currentField.WriteRune(r)
		} else if r == ' ' && !inQuote {
			if currentField.Len() > 0 {
				fields = append(fields, currentField.String())
				currentField.Reset()
			}
		} else {
			currentField.WriteRune(r)
		}
	}

	if currentField.Len() > 0 {
		fields = append(fields, currentField.String())
	}

	for _, field := range fields {
		if idx := strings.Index(field, "="); idx >= 0 {
			key := field[:idx]
			value := field[idx+1:]
			result[key] = value
		}
	}

	return result
}

func parseLogOutput(t *testing.T, output string) map[string]map[string]string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := make(map[string]map[string]string)

	for _, line := range lines {
		fields := parseLogLine(t, line)
		if timestamp, ok := fields["time"]; ok {
			result[timestamp] = fields
		}
	}

	return result
}

func compareLogFields(t *testing.T, timestamp string, origFields, replayFields map[string]string) {
	t.Helper()

	for field, origValue := range origFields {
		if replayValue, ok := replayFields[field]; ok {
			if origValue != replayValue {
				t.Errorf("Field %q has different values for timestamp %s: original=%q, replayed=%q",
					field, timestamp, origValue, replayValue)
			}
		}
	}
}

// Test helper types
type errorHandler struct{}

func (h *errorHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }
func (h *errorHandler) Handle(ctx context.Context, r slog.Record) error    { return io.ErrUnexpectedEOF }
func (h *errorHandler) WithAttrs(attrs []slog.Attr) slog.Handler           { return h }
func (h *errorHandler) WithGroup(name string) slog.Handler                 { return h }

type sleepyHandler struct {
	sleepTime time.Duration
}

func (h *sleepyHandler) Enabled(ctx context.Context, level slog.Level) bool { return true }
func (h *sleepyHandler) Handle(ctx context.Context, r slog.Record) error {
	time.Sleep(h.sleepTime)
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
func (h *sleepyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *sleepyHandler) WithGroup(name string) slog.Handler       { return h }

type discardWriter struct{}

func (d *discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
