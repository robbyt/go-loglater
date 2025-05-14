# LogLater

LogLater is a Go library for capturing and replaying structured logs from the standard library's `log/slog` package. It provides a clean interface for testing, debugging, and analyzing log output.

## Features

- Seamlessly integrates with Go's `log/slog` package
- Thread-safe collection of log records
- Capture logs during tests or normal operation
- Replay captured logs to any `slog.Handler`
- Store and retrieve logs for analysis

## Installation

```bash
go get github.com/robbyt/go-loglater
```

## Usage

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
    
    // Create our collector with the text handler as the base
    collector := loglater.NewLogCollector(textHandler)
    
    // Create a logger that uses our collector
    logger := slog.New(collector)
    
    // Log some events (these will be collected but not output yet)
    logger.Info("Starting application", "version", "1.0.0")
    logger.Warn("Configuration file not found, using defaults")
    logger.Error("Failed to connect to database", "error", "connection timeout")
    
    fmt.Println("Logs have been collected but not yet output.")
    fmt.Println("Now playing logs:")
    
    // Now output all the collected logs to the same handler
    err := collector.PlayLogs(textHandler)
    if err != nil {
        fmt.Printf("Error playing logs: %v\n", err)
    }
}
```

## License

Apache License 2.0

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.