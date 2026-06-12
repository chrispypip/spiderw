//go:build stress

package logging_test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/logging"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestStress_Logging_LargeAttributeLists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large attribute stress test in short mode")
	}

	ctx := context.Background()
	tl := logging.NewTestLogger()

	// Build a very large attribute list
	const attrCount = 1_000
	attrs := make([]any, 0, attrCount*2)
	for i := range attrCount {
		attrs = append(attrs, fmt.Sprintf("k%d", i), i)
	}

	var wg sync.WaitGroup

	// Hammer With() + logging concurrently
	for i := range 100 {
		wg.Go(func() {
			l := tl.With(attrs...)
			l.Info(ctx, "large-attr-log", "goroutine", i)
		})
	}

	wg.Wait()

	out := tl.String()
	require.NotEmpty(t, out)
}

func TestStress_Logging_DeepContextNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping deep context nesting stress test in short mode")
	}

	base := logging.NewTestLogger()
	ctx := logging.WithLogger(context.Background(), base)

	const depth = 200

	var last *logging.TestLogger

	// Build deeply nested contexts
	for i := range depth {
		l := logging.NewTestLogger().With("depth", i)
		if tl, ok := l.(*logging.TestLogger); ok {
			last = tl
		}
		ctx = logging.WithLogger(ctx, l)
	}

	require.NotNil(t, last, "expected final logger to exist")

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Go(func() {
			logging.FromContext(ctx).Info(ctx, "deep-context-log", "i", i)
		})
	}

	wg.Wait()

	// Logs should go to the *final* logger
	require.NotEmpty(t, last.String())

	// Base logger should remain untouched
	require.Empty(t, base.String())
}

// Stress test that hammer-mixes:
//   - testlogger
//   - stdlogger (text & JSON)
//   - noplogger
func TestStress_Logging_MixedLogger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mixed logger stress test in short mode")
	}

	ctx := context.Background()
	buf := &syncBuffer{}

	textLogger := logging.New(logging.Config{
		Writer: buf,
		JSON:   false,
	})
	jsonLogger := logging.New(logging.Config{
		Writer: buf,
		JSON:   true,
	})
	nopLogger := logging.Nop()

	var wg sync.WaitGroup
	// Run a large number of goroutines mixing logging types & operations.
	for i := range 300 {
		wg.Go(func() {
			switch i % 3 {
			case 0:
				textLogger.Info(ctx, "text-info", "i", i)
				textLogger.Warn(ctx, "text-warn", "i", i)
			case 1:
				jsonLogger.Info(ctx, "json-info", "i", i)
				jsonLogger.Error(ctx, "json-error", "i", i)
			default:
				nopLogger.Debug(ctx, "nop-debug", "i", i)
			}
		})
	}

	wg.Wait()

	out := buf.String()
	require.NotEmpty(t, out)

}

// Stress test for logging inside context replacement loops (realistic pattern).
func TestStress_Logging_MixedLogger_ContextSwapStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping context swap stress test in short mode")
	}

	baseBuf := &syncBuffer{}
	baseLogger := logging.New(logging.Config{
		Writer: baseBuf,
		JSON:   false,
	})

	ctx := context.Background()
	ctxWithLogger := logging.WithLogger(ctx, baseLogger)

	var wg sync.WaitGroup
	for i := range 150 {
		wg.Go(func() {
			localLogger := logging.New(logging.Config{
				Writer: baseBuf,
				JSON:   i%2 == 0,
			})
			localCtx := logging.WithLogger(ctxWithLogger, localLogger)
			l := logging.FromContext(localCtx)
			l.Info(localCtx, "context-swap", "i", i)
		})
	}

	wg.Wait()
}

// Stress test: deeply nested With() calls across mixed logger types
func TestStress_Logging_MixedLogger_DeepWithChains(t *testing.T) {
	ctx := context.Background()
	loggers := mixedLoggerSources()

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Go(func() {
			// Pick logger type based on iteration to force intermixing
			l := loggers[i%len(loggers)]

			// Deep chain
			for j := range 20 {
				l = l.With("lvl", j)
				l.Debug(ctx, "deep chain", "i", i, "j", j)
			}
		})
	}

	wg.Wait()
}

// Stress test mixing huge concurrency + pauses (simulates real network/dbus workloads)
func TestStress_Logging_MixedLogger_PauseAndBurst(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping pause-and-burts test in short mode")
	}

	ctx := context.Background()
	buf := &syncBuffer{}

	textLogger := logging.New(logging.Config{
		Writer: buf,
		JSON:   false,
	})
	jsonLogger := logging.New(logging.Config{
		Writer: buf,
		JSON:   true,
	})

	var wg sync.WaitGroup
	for i := range 200 {
		wg.Go(func() {
			if i%2 == 0 {
				textLogger.Info(ctx, "burst-text", "i", i)
				textLogger.Warn(ctx, "burst-warn", "i", i)
			} else {
				jsonLogger.Info(ctx, "burst-json", "i", i)
				jsonLogger.Error(ctx, "burst-json-error", "i", i)
			}
		})
	}

	wg.Wait()

	time.Sleep(10 * time.Millisecond)
	for i := range 100 {
		wg.Go(func() {
			if i%2 == 0 {
				textLogger.Info(ctx, "second-burst-text", "i", i)
			} else {
				jsonLogger.Info(ctx, "second-burst-json", "i", i)
			}
		})
	}

	wg.Wait()

	out := buf.String()
	require.NotEmpty(t, out)
}

// mixedLoggerSources returns a pool of different logger implementations
// that stress tests can shuffle between.
func mixedLoggerSources() []logging.Logger {
	var buf bytes.Buffer

	return []logging.Logger{
		logging.Nop(),
		logging.NewTestLogger(),
		logging.New(logging.Config{Writer: &buf}),             // stdlogger text
		logging.New(logging.Config{Writer: &buf, JSON: true}), // stdlogger JSON
	}
}
