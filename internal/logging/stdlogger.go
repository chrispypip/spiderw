package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Config allows customizing output, format, and log level.
type Config struct {
	// Writer receives log output. If nil, os.Stderr is used.
	Writer io.Writer

	// Level controls the minimum enabled log level. If nil, slog.LevelInfo is used.
	Level slog.Leveler

	// JSON selects JSON output when true and text output when false.
	JSON bool
}

// stdLogger wraps slog.Logger and implements Logger.
type stdLogger struct {
	l *slog.Logger
}

// Debug writes a debug log entry.
func (s *stdLogger) Debug(ctx context.Context, msg string, args ...any) {
	s.l.Log(ctx, slog.LevelDebug, msg, args...)
}

// Info writes an info log entry.
func (s *stdLogger) Info(ctx context.Context, msg string, args ...any) {
	s.l.Log(ctx, slog.LevelInfo, msg, args...)
}

// Warn writes a warn log entry.
func (s *stdLogger) Warn(ctx context.Context, msg string, args ...any) {
	s.l.Log(ctx, slog.LevelWarn, msg, args...)
}

// Error writes an error log entry.
func (s *stdLogger) Error(ctx context.Context, msg string, args ...any) {
	s.l.Log(ctx, slog.LevelError, msg, args...)
}

// With returns a logger enriched with key-value fields.
func (s *stdLogger) With(args ...any) Logger {
	return &stdLogger{l: s.l.With(args...)}
}

// New returns a slog-backed Logger that uses the given config.
func New(cfg Config) Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stderr
	}
	lvl := cfg.Level
	if lvl == nil {
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return &stdLogger{
		l: slog.New(handler),
	}
}
