# LogLater

[![Go Reference](https://pkg.go.dev/badge/github.com/robbyt/go-loglater.svg)](https://pkg.go.dev/github.com/robbyt/go-loglater)
[![Go Report Card](https://goreportcard.com/badge/github.com/robbyt/go-loglater)](https://goreportcard.com/report/github.com/robbyt/go-loglater)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=robbyt_go-loglater&metric=coverage)](https://sonarcloud.io/summary/new_code?id=robbyt_go-loglater)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

LogLater is a Go library for capturing and replaying structured logs from the standard library's `log/slog` package. It implements the `slog.Handler` interface to collect log entries, store them, and replay them later.

## Key Benefits

- **Zero Dependencies**: Uses only the Go standard library
- **Stdlib Integration**: Works with the standard `log/slog` package as a `slog.Handler` implementation shim
- **Format Flexibility**: Collect once, replay to any handler (text, JSON, custom)
- **Concurrency Support**: Safe for use in multi-goroutine applications
- **Customize your output**: Capture, replay, or save logs with control over output timing

## Installation

```bash
go get github.com/robbyt/go-loglater@latest
```

## Usage

### Basic Usage

```go
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/robbyt/go-loglater"
)

func main() {
    // Create a text handler that outputs to stdout
    textHandler := slog.NewTextHandler(os.Stdout, nil)
    
    // Create collector with the stdout text handler as the base handler
    collector := loglater.NewLogCollector(textHandler)
    
    // Create a logger that uses our collector shim
    logger := slog.New(collector)
    
    // Log some events (these will be output immediately AND collected)
    logger.Info("Starting application", "version", "1.0.0")
    logger.Warn("Configuration file not found")
    logger.Error("Failed to connect to database", "error", "timeout")
    
    fmt.Println("Now replaying logs:")
    
    // Replay all the collected logs to the same handler
    err := collector.PlayLogs(textHandler)
    if err != nil {
        fmt.Printf("Error playing logs: %v\n", err)
    }
}
```

### Deferred Logging

To collect logs without immediately printing them:

```go
// Create a collector with no output handler
collector := loglater.NewLogCollector(nil)

// Create a logger that uses our collector
logger := slog.New(collector)

// Log some events (these will only be collected, not output)
logger.Info("This log is just stored, not output")

// Later, play logs to stdout with another handler
textHandler := slog.NewTextHandler(os.Stdout, nil)
collector.PlayLogs(textHandler)
```

### Working with Groups

LogLater preserves group structure when replaying logs:

```go
collector := loglater.NewLogCollector(nil)
logger := slog.New(collector)

// Create loggers with groups
dbLogger := logger.WithGroup("db")
apiLogger := logger.WithGroup("api")

// Log with different loggers
dbLogger.Info("Connected to database", "host", "db.example.com")
apiLogger.Error("API request failed", "endpoint", "/users", "status", 500)

// Play logs to JSON handler - group structure is preserved
jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
collector.PlayLogs(jsonHandler)
```

### Cleanup Options

LogLater supports automatic cleanup of old log records through storage options:

```go
import (
    "context"
    "github.com/robbyt/go-loglater"
    "github.com/robbyt/go-loglater/storage"
)

// Create storage with a maximum size of 1000 records
store := storage.NewRecordStorage(storage.WithMaxSize(1000))
collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))

// Create storage that keeps only logs from the last hour
store := storage.NewRecordStorage(
    storage.WithMaxAge(1 * time.Hour)
)
collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))

// With asynchronous cleanup (uses background goroutine)
store := storage.NewRecordStorage(
    storage.WithMaxSize(1000),
    storage.WithAsyncCleanup(true),
    storage.WithDebounceTime(500 * time.Millisecond) // Customize debounce time
)
collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))

// With cancellable context for the cleanup worker
ctx, cancel := context.WithCancel(context.Background())
store := storage.NewRecordStorage(
    storage.WithContext(ctx),
    storage.WithMaxSize(1000),
    storage.WithAsyncCleanup(true)
)
collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))
// Cancel the context to stop async cleanup (forever)
cancel()

// With a custom cleanup function
customCleanup := func(records []storage.Record) []storage.Record {
    // Keep only error logs
    var result []storage.Record
    for _, r := range records {
        if r.Level >= slog.LevelError {
            result = append(result, r)
        }
    }
    return result
}

store := storage.NewRecordStorage(storage.WithCleanupFunc(customCleanup))
collector := loglater.NewLogCollector(nil, loglater.WithStorage(store))
```

## License

Apache License 2.0
