package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/robbyt/go-loglater"
	"github.com/robbyt/go-loglater/storage"
)

// demoSizeLimitedLogs demonstrates a size-limited log collection
func demoSizeLimitedLogs() (*loglater.LogCollector, error) {
	// Create storage with maximum size of 3 records
	store := storage.NewRecordStorage(storage.WithMaxSize(3))

	// Create a collector with the configured storage
	collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))
	logger := slog.New(collector)

	// Log 5 messages, but only the most recent 3 will be kept
	for i := range 5 {
		i++
		logger.Info(fmt.Sprintf("Message %d", i))
	}

	return collector, nil
}

// demoAgeLimitedLogs demonstrates a time-based log cleanup
func demoAgeLimitedLogs() (*loglater.LogCollector, error) {
	// Create storage with 1h age limit and async cleanup
	store := storage.NewRecordStorage(
		storage.WithMaxAge(1*time.Hour),
		storage.WithAsyncCleanup(true),
		storage.WithDebounceTime(100*time.Millisecond), // Short debounce for testing
	)

	// Create a collector with the configured storage
	collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))
	logger := slog.New(collector)

	// Log messages with different times (in normal usage, these would have current timestamps)
	// Only in tests we'll use mock test records with specific times
	logger.Info("This is an old message")
	logger.Info("This is a recent message")
	logger.Info("This is a current message")

	// Wait a moment for async cleanup
	time.Sleep(200 * time.Millisecond)

	return collector, nil
}

// demoCustomCleanup demonstrates using a custom cleanup function
func demoCustomCleanup() (*loglater.LogCollector, error) {
	// Create storage with custom cleanup
	store := storage.NewRecordStorage(storage.WithCleanupFunc(createErrorFilter()))

	// Create collector with the configured storage
	collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))
	logger := slog.New(collector)

	// Log different levels
	logger.Info("This is an info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message", "error", "something failed")

	return collector, nil
}

// createErrorFilter creates a cleanup function that only keeps error logs
func createErrorFilter() storage.CleanupFunc {
	return func(records []storage.Record) []storage.Record {
		var result []storage.Record
		for _, r := range records {
			if r.Level >= slog.LevelError {
				result = append(result, r)
			}
		}
		return result
	}
}

func main() {
	fmt.Println("=== Example 1: Size-limited log collection ===")
	collector, err := demoSizeLimitedLogs()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Display the logs that were kept
	logs := collector.GetLogs()
	fmt.Printf("Collected logs (should be only 3): %d\n", len(logs))

	// Play the logs
	textHandler := slog.NewTextHandler(os.Stdout, nil)
	if err := collector.PlayLogs(textHandler); err != nil {
		fmt.Printf("Error playing logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Example 2: Age-limited log collection ===")
	collector, err = demoAgeLimitedLogs()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Display the logs that were kept
	logs = collector.GetLogs()
	fmt.Printf("Collected logs (should filter out old ones): %d\n", len(logs))

	// Play the logs
	if err := collector.PlayLogs(textHandler); err != nil {
		fmt.Printf("Error playing logs: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Example 3: Custom cleanup function ===")
	collector, err = demoCustomCleanup()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Display the logs that were kept
	logs = collector.GetLogs()
	fmt.Printf("Collected logs (should only be errors): %d\n", len(logs))

	// Play the logs
	if err := collector.PlayLogs(textHandler); err != nil {
		fmt.Printf("Error playing logs: %v\n", err)
		os.Exit(1)
	}
}
