//go:build race

package logging_test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/logging"
)

// TestRace_Logging() ensures that the logger implementation is safe under
// concurrency when run with "go test -race".
func TestRace_Logging(t *testing.T) {
	tl := logging.NewTestLogger()
	ctx := logging.WithLogger(context.Background(), tl)

	// Hammer basic logging functions.
	t.Run("basic concurrent logging", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := range 100 {
			wg.Go(func() {
				l := logging.FromContext(ctx)
				l.Debug(ctx, "debug message", "i", i)
				l.Info(ctx, "info message", "i", i)
				l.Warn(ctx, "warn message", "i", i)
				l.Error(ctx, "error message", "i", i)
			})
		}

		wg.Wait()
	})

	// Hammer With(...) logging.
	t.Run("concurrent With() usage", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := range 100 {
			wg.Go(func() {
				l := logging.FromContext(ctx)

				child := l.With("child", i)
				child.Info(ctx, "child logger message")
			})
		}

		wg.Wait()
	})

	// Hammer context switching.
	t.Run("context switching under load", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := range 100 {
			wg.Go(func() {
				local := logging.WithLogger(ctx, tl.With("local", i))
				logging.FromContext(local).Info(local, "context write")
			})
		}

		wg.Wait()
	})

	t.Run("deep With() chains", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := range 200 {
			wg.Go(func() {
				l := logging.FromContext(ctx)
				for j := range 10 {
					// Every With() returns a *new* TestLogger wrapper
					l = l.With("lvl", j)
					l.Info(ctx, "deep chain", "i", i)
				}
			})
		}

		wg.Wait()
	})

	out := tl.String()
	require.NotEmpty(t, out)
}

func TestRace_Logging_StdLogger(t *testing.T) {
	t.Run("concurrent writes", func(t *testing.T) {
		var buf bytes.Buffer
		l := logging.New(logging.Config{Writer: &buf})
		ctx := context.Background()

		var wg sync.WaitGroup
		for i := range 200 {
			wg.Go(func() {
				l.Info(ctx, "msg", "i", i)
			})
		}
		wg.Wait()
	})

	t.Run("concurrent With() chains", func(t *testing.T) {
		var buf bytes.Buffer
		l := logging.New(logging.Config{Writer: &buf})
		ctx := context.Background()

		var wg sync.WaitGroup
		for i := range 200 {
			wg.Go(func() {
				child := l
				for j := range 10 {
					child = child.With("lvl", j)
					child.Debug(ctx, "nested", "i", i)
				}
			})
		}
		wg.Wait()
	})
}

func TestRace_Logging_TestLogger(t *testing.T) {
	tl := logging.NewTestLogger()
	ctx := context.Background()

	t.Run("concurrent writes", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range 200 {
			wg.Go(func() {
				tl.Info(ctx, "test message", "i", i)
			})
		}
		wg.Wait()
	})

	t.Run("concurrent String() calls", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range 100 {
			wg.Add(2)
			go func() {
				defer wg.Done()
				_ = tl.String()
			}()
			go func(i int) {
				defer wg.Done()
				tl.Debug(ctx, "msg", "i", i)
			}(i)
		}
		wg.Wait()
	})

	t.Run("child loggers sharing buffer", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := range 200 {
			wg.Go(func() {
				child := tl.With("child", i)
				for j := range 5 {
					child.Warn(ctx, "child chain", "j", j)
				}
			})
		}

		wg.Wait()
	})
}

func TestRace_Logging_NopLogger(t *testing.T) {
	l := logging.Nop()
	ctx := context.Background()

	t.Run("parallel hammering", func(t *testing.T) {
		var wg sync.WaitGroup

		for i := range 500 {
			wg.Go(func() {
				// None of these should race or panic
				l.Debug(ctx, "ignored", "i", i)
				l.Info(ctx, "ignored", "i", i)
				l.Warn(ctx, "ignored", "i", i)
				l.Error(ctx, "ignored", "i", i)

				_ = l.With("x", i)
			})
		}

		wg.Wait()
	})
}
