package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/robbyt/go-loglater"
)

// demoGroupLogging demonstrates logging with groups using LogLater
func demoGroupLogging(w io.Writer) (*loglater.LogCollector, error) {
	textHandler := slog.NewTextHandler(w, nil)
	collector := loglater.NewLogCollector(textHandler)
	logger := slog.New(collector)

	// Create loggers with different groups
	dbLogger := logger.WithGroup("db").With("component", "database")
	apiLogger := logger.WithGroup("api").With("component", "http")

	// Log with the different loggers
	logger.Info("Service started", "version", "2.1.0")
	dbLogger.Info("Connected to database", "host", "db.example.com")
	dbLogger.Error("Query failed", "error", "timeout", "query", "SELECT * FROM users")
	apiLogger.Info("HTTP server listening", "port", 8080)
	apiLogger.Warn("Rate limit exceeded", "client", "192.168.1.42", "endpoint", "/api/users")

	return collector, nil
}

func main() {
	fmt.Println("=== Example 1: Text output with groups ===")
	collector, err := demoGroupLogging(os.Stdout)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	textHandler := slog.NewTextHandler(os.Stdout, nil)
	fmt.Println("\nReplaying logs to the same handler:")
	if err := collector.PlayLogs(textHandler); err != nil {
		fmt.Printf("Error replaying logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nLog summary:")
	logs := collector.GetLogs()
	for i, log := range logs {
		fmt.Printf("Log %d: [%s] %s\n", i+1, log.Level, log.Message)
	}
	fmt.Printf("Total log count: %d\n\n", len(logs))

	fmt.Println("=== Example 2: JSON output with deferred logging ===")
	// Create a collector with no immediate output
	collector = loglater.NewLogCollector(nil)
	logger := slog.New(collector)

	// Create loggers with groups
	dbLogger := logger.WithGroup("db")
	apiLogger := logger.WithGroup("api")

	// Log with different loggers (nothing output yet)
	dbLogger.Info("Connected to database", "host", "db.example.com")
	apiLogger.Error("API request failed", "endpoint", "/users", "status", 500)

	// Play logs to a JSON handler - structure is preserved
	fmt.Println("Playing logs to JSON handler (group structure is preserved):")
	jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
	if err := collector.PlayLogs(jsonHandler); err != nil {
		fmt.Printf("Error replaying logs to JSON handler: %v\n", err)
		os.Exit(1)
	}
}
