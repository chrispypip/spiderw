package logging

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
)

// TestLogger is a logger used in unit tests.
// Logs are stored into an internal buffer for assertions.
type TestLogger struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
	std Logger
}

// NewTestLogger returns a test logger that writes to an internal buffer.
func NewTestLogger() *TestLogger {
	buf := &bytes.Buffer{}
	mu := &sync.Mutex{}

	std := New(Config{
		Writer: buf,
		Level:  slog.LevelDebug, // capture everything
		JSON:   false,
	})

	return &TestLogger{
		mu:  mu,
		buf: buf,
		std: std,
	}
}

// Debug writes a debug log entry.
func (t *TestLogger) Debug(ctx context.Context, msg string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.std.Debug(ctx, msg, args...)
}

// Info writes an info log entry.
func (t *TestLogger) Info(ctx context.Context, msg string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.std.Info(ctx, msg, args...)
}

// Warn writes a warn log entry.
func (t *TestLogger) Warn(ctx context.Context, msg string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.std.Warn(ctx, msg, args...)
}

// Error writes an error log entry.
func (t *TestLogger) Error(ctx context.Context, msg string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.std.Error(ctx, msg, args...)
}

// With returns a logger enriched with key-value fields.
func (t *TestLogger) With(args ...any) Logger {
	t.mu.Lock()
	defer t.mu.Unlock()

	// stdlogger.With returns a new logger, so wrap it again
	return &TestLogger{
		mu:  t.mu,
		buf: t.buf,
		std: t.std.With(args...),
	}
}

// String returns all accumulated logs as a string.
func (t *TestLogger) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

// Bytes returns the raw log buffer.
func (t *TestLogger) Bytes() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.buf.Bytes()...)
}
