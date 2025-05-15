package loglater

// Option defines a function type for configuring LogCollector
type Option func(*LogCollector)

// WithStorage allows specifying a custom storage implementation, or a storage implementation with custom options.
func WithStorage(store Storage) Option {
	return func(lc *LogCollector) {
		lc.store = store
	}
}
