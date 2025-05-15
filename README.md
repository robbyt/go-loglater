# LogLater

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

## License

Apache License 2.0
