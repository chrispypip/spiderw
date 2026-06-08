package logging

import (
	"context"
)

// Logger is the interface used by spiderw logging call sites. It matches a thin
// subset of slog behavior so callers can swap implementations.
type Logger interface {
	// Debug writes a debug log entry.
	Debug(ctx context.Context, msg string, args ...any)

	// Info writes an info log entry.
	Info(ctx context.Context, msg string, args ...any)

	// Warn writes a warning log entry.
	Warn(ctx context.Context, msg string, args ...any)

	// Error writes an error log entry.
	Error(ctx context.Context, msg string, args ...any)

	// With returns a new Logger enriched with attributes.
	With(args ...any) Logger
}

type ctxKey struct{}

// WithLogger returns a context that stores l. If l is nil, it stores a no-op
// logger.
func WithLogger(ctx context.Context, l Logger) context.Context {
	if l == nil {
		l = Nop()
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves a Logger from ctx. If none is stored, it returns a
// no-op logger.
func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return Nop()
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if l, ok := v.(Logger); ok && l != nil {
			return l
		}
	}
	return Nop()
}
