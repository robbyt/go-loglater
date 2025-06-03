package storage

import "log/slog"

// OperationType represents the type of handler operation.
type OperationType int

const (
	OpAttrs OperationType = iota
	OpGroup
)

// Operation represents a single handler modification operation.
// This captures either a WithAttrs() or WithGroup() call made on a slog.Handler,
// preserving the exact order of operations for accurate replay.
//
// Example Journal for: logger.With("global", "value").WithGroup("api").With("user", "123")
//  1. Operation{Type: OpAttrs, Attrs: [global=value]}
//  2. Operation{Type: OpGroup, Group: "api"}
//  3. Operation{Type: OpAttrs, Attrs: [user=123]}
//
// During replay, this ensures "global" stays global while "user" gets grouped as "api.user".
type Operation struct {
	Type  OperationType
	Attrs []slog.Attr // for WithAttrs operations
	Group string      // for WithGroup operations
}

// OperationJournal represents the journal of WithAttrs and WithGroup operations
// that were applied to create a particular logger instance.
//
// This journal is stored alongside each log record and replayed during log output
// to ensure the exact same handler state is reconstructed, preserving the correct
// relationship between global attributes (added before groups) and grouped attributes
// (added after groups).
//
// Without this journal, attributes could be incorrectly grouped during replay,
// causing "global=value" to become "group.global=value".
type OperationJournal []Operation
