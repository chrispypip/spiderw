package logging

import "context"

// nopLogger discards all logs and does not allocate.
type nopLogger struct{}

// Debug writes a debug log entry.
func (nopLogger) Debug(ctx context.Context, msg string, args ...any) {}

// Info writes an info log entry.
func (nopLogger) Info(ctx context.Context, msg string, args ...any) {}

// Warn writes a warn log entry.
func (nopLogger) Warn(ctx context.Context, msg string, args ...any) {}

// Error writes an error log entry.
func (nopLogger) Error(ctx context.Context, msg string, args ...any) {}

// With returns a logger enriched with key-value fields.
func (n nopLogger) With(...any) Logger { return n }

// Nop returns a logger that discards all log messages.
func Nop() Logger {
	return nopLogger{}
}
