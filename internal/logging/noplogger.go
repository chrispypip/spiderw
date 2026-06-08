package logging

import "context"

// nopLogger discards all logs and does not allocate.
type nopLogger struct{}

// Debug writes a debug log entry.
func (nopLogger) Debug(context.Context, string, ...any) {}

// Info writes an info log entry.
func (nopLogger) Info(context.Context, string, ...any) {}

// Warn writes a warn log entry.
func (nopLogger) Warn(context.Context, string, ...any) {}

// Error writes an error log entry.
func (nopLogger) Error(context.Context, string, ...any) {}

// With returns a logger enriched with key-value fields.
func (n nopLogger) With(...any) Logger { return n }

// Nop returns a logger that discards all log messages.
func Nop() Logger {
	return nopLogger{}
}
