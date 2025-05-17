package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/robbyt/go-loglater"
)

// LogDemo demonstrates the basic functionality of the loglater package.
// It creates a log collector, logs several messages, and then replays them.
// It also shows how loglater can work with both immediate and deferred logging patterns.
//
// Parameters:
//   - handler: An slog.Handler to output logs. If nil, logs are only collected until replay.
func LogDemo(handler slog.Handler) (int, error) {
	// Create our collector with the handler as the base
	collector := loglater.NewLogCollector(handler)

	// Create a logger that uses our collector
	logger := slog.New(collector)

	// Log some events (these will be collected and only output if handler != nil)
	logger.Info("Starting demo", "version", "1.0.0")
	logger.Warn("Just a warning message")
	logger.Error("This is an error message", "error", "oops!")

	if handler == nil {
		fmt.Println("Logs have been collected but not yet output.")
		fmt.Println("Now playing logs to stdout:")

		// For nil handler case, create a handler for playback
		playbackHandler := slog.NewTextHandler(os.Stdout, nil)
		if err := collector.PlayLogs(playbackHandler); err != nil {
			return 0, fmt.Errorf("error playing logs: %w", err)
		}
	} else {
		fmt.Println("Now replaying logs:")
		// For non-nil handler case, replay to the same handler
		if err := collector.PlayLogs(handler); err != nil {
			return 0, fmt.Errorf("error playing logs: %w", err)
		}
	}

	fmt.Println("\nLog summary:")
	logCount := 0
	for i, log := range collector.GetLogs() {
		logCount++
		fmt.Printf("Log %d: [%s] %s\n", i+1, log.Level, log.Message)
	}

	return logCount, nil
}

func main() {
	// Example 1: Passing a real handler - logs appear immediately and when replayed
	fmt.Println("=== Example 1: With immediate logging ===")
	textHandler := slog.NewTextHandler(os.Stdout, nil)
	logCount, err := LogDemo(textHandler)
	if err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Example 1 completed with %d logs\n\n", logCount)

	// Example 2: Passing nil - logs are only collected, not displayed until playback
	fmt.Println("=== Example 2: With deferred logging ===")
	logCount, err = LogDemo(nil)
	if err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Example 2 completed with %d logs\n", logCount)
}
