package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/robbyt/go-loglater"
)

// ServiceLogger simulates a service that uses grouped/attributed loggers
type ServiceLogger struct {
	baseLogger *slog.Logger
	dbLogger   *slog.Logger
	apiLogger  *slog.Logger
	collector  *loglater.LogCollector
}

// NewServiceLogger creates a new service logger with collector
func NewServiceLogger(handler slog.Handler) *ServiceLogger {
	// Create the collector with the handler
	collector := loglater.NewLogCollector(handler)

	// Create a base logger
	baseLogger := slog.New(collector)

	// Create specialized loggers that share the same collector
	dbLogger := baseLogger.WithGroup("db").With("component", "database")
	apiLogger := baseLogger.WithGroup("api").With("component", "http")

	return &ServiceLogger{
		baseLogger: baseLogger,
		dbLogger:   dbLogger,
		apiLogger:  apiLogger,
		collector:  collector,
	}
}

// LogServiceActivity simulates normal activity with different loggers
func (s *ServiceLogger) LogServiceActivity() {
	// Log from main service
	s.baseLogger.Info("Service started", "version", "2.1.0")

	// Log from database component
	s.dbLogger.Info("Connected to database", "host", "db.example.com")
	s.dbLogger.Error("Query failed", "error", "timeout", "query", "SELECT * FROM users")

	// Log from API component
	s.apiLogger.Info("HTTP server listening", "port", 8080)
	s.apiLogger.Warn("Rate limit exceeded", "client", "192.168.1.42", "endpoint", "/api/users")
}

// GroupLogDemo demonstrates how LogLater captures logs from different logger
// instances that share the same underlying collector
func GroupLogDemo(handler slog.Handler) (int, error) {
	// Create service with multiple loggers
	fmt.Println("Creating service with specialized loggers (base, db, api)...")
	service := NewServiceLogger(handler)

	// Generate logs from different components
	fmt.Println("Logging events from different components...")
	service.LogServiceActivity()

	// Show summary of captured logs
	if handler == nil {
		fmt.Println("\nLogs have been collected but not yet output.")
		fmt.Println("Now playing logs to stdout:")

		// For nil handler case, create a handler for playback
		playbackHandler := slog.NewTextHandler(os.Stdout, nil)
		if err := service.collector.PlayLogs(playbackHandler); err != nil {
			return 0, fmt.Errorf("error playing logs: %w", err)
		}
	} else {
		fmt.Println("\nNow replaying logs:")
		// For non-nil handler case, replay to the same handler
		if err := service.collector.PlayLogs(handler); err != nil {
			return 0, fmt.Errorf("error playing logs: %w", err)
		}
	}

	// Display log summary
	fmt.Println("\nLog summary:")
	logCount := 0
	for i, log := range service.collector.GetLogs() {
		logCount++
		fmt.Printf("Log %d: [%s] %s\n", i+1, log.Level, log.Message)
		// Show a sample of attributes for the first few logs to demonstrate groups
		if i < 3 && len(log.Attrs) > 0 {
			fmt.Printf("  Attributes: ")
			for j, attr := range log.Attrs {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Printf("%s: %v", attr.Key, attr.Value)
			}
			fmt.Println()
		}
	}

	return logCount, nil
}

func main() {
	// Example 1: Passing a real handler - logs appear immediately and when replayed
	fmt.Println("=== Example 1: With immediate logging ===")
	textHandler := slog.NewTextHandler(os.Stdout, nil)
	logCount, err := GroupLogDemo(textHandler)
	if err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Example 1 completed with %d logs\n\n", logCount)

	// Example 2: Passing nil - logs are only collected, not displayed until playback
	fmt.Println("=== Example 2: With deferred logging ===")
	logCount, err = GroupLogDemo(nil)
	if err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Example 2 completed with %d logs\n", logCount)
}
