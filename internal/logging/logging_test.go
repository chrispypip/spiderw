//go:build unit

package logging_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/logging"
)

func TestLogging(t *testing.T) {
	t.Parallel()

	t.Run("Context", func(t *testing.T) {
		t.Parallel()

		t.Run("WithLoggerAndFromContext_ReturnsInjectedLogger", func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			tl := logging.NewTestLogger()

			ctx2 := logging.WithLogger(ctx, tl)
			got := logging.FromContext(ctx2)
			require.NotNil(t, got)

			got.Info(ctx2, "hello world")
			require.Contains(t, tl.String(), "hello world")
		})

		t.Run("FromContext_DefaultsToNop", func(t *testing.T) {
			t.Parallel()

			l := logging.FromContext(context.Background())
			require.NotNil(t, l)

			// Should not panic.
			l.Info(context.Background(), "ignored")
		})

		t.Run("WithLogger_OverwritesExistingLogger", func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			tl1 := logging.NewTestLogger()
			tl2 := logging.NewTestLogger()
			require.NotNil(t, tl1)
			require.NotNil(t, tl2)

			ctx1 := logging.WithLogger(ctx, tl1)
			ctx2 := logging.WithLogger(ctx1, tl2)

			got := logging.FromContext(ctx2)
			got.Info(ctx2, "hello")
			require.NotContains(t, tl1.String(), "hello")
			require.Contains(t, tl2.String(), "hello")
		})
	})

	t.Run("Nop", func(t *testing.T) {
		t.Parallel()

		l := logging.Nop()

		// Should not panic.
		l.Debug(context.Background(), "ignored")
		l.Info(context.Background(), "ignored")
		l.Warn(context.Background(), "ignored")
		l.Error(context.Background(), "ignored")

		l2 := l.With("something", 123)

		// Should also not panic.
		l2.Info(context.Background(), "still ignored")
	})

	t.Run("Std", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()

		t.Run("Construction_ReturnsLogger", func(t *testing.T) {
			t.Parallel()

			l := logging.New(logging.Config{Writer: &bytes.Buffer{}})
			require.NotNil(t, l)
		})

		t.Run("Text", func(t *testing.T) {
			t.Parallel()

			t.Run("OutputIncludesMsgAndAttributes", func(t *testing.T) {
				t.Parallel()

				var buf bytes.Buffer
				l := logging.New(logging.Config{JSON: false, Writer: &buf})
				require.NotNil(t, l)

				child := l.With("k", 1)
				child.Info(ctx, "msg")

				out := buf.String()
				require.NotEmpty(t, out)
				require.Contains(t, out, "msg")
				require.Contains(t, out, "k=1")            // attribute included
				require.NotContains(t, out, `"msg":"msg"`) // JSON style
				require.Contains(t, out, "msg=msg")        // text style
			})

			t.Run("WritesToBuffer", func(t *testing.T) {
				t.Parallel()

				var buf bytes.Buffer
				l := logging.New(logging.Config{JSON: false, Writer: &buf})

				l.Info(ctx, "hello", "x", 1)
				out := buf.String()
				require.NotEmpty(t, out)
				require.Contains(t, out, "hello")
				require.Contains(t, out, "x=1")
			})

			t.Run("With_InheritsAttributes", func(t *testing.T) {
				t.Parallel()

				var buf bytes.Buffer
				l := logging.New(logging.Config{JSON: false, Writer: &buf})

				child := l.With("mod", "daemon")
				child.Info(ctx, "start")
				out := buf.String()
				require.NotEmpty(t, out)
				require.Contains(t, out, "start")
				require.Contains(t, out, "mod=daemon")
			})
		})

		t.Run("JSON", func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			l := logging.New(logging.Config{JSON: true, Writer: &buf})

			l.Info(ctx, "json-test", "k", 123)
			out := buf.String()
			require.NotEmpty(t, out)
			require.Contains(t, out, `"msg":"json-test"`)
			require.Contains(t, out, `"k":123`, "expected numeric attribute")
		})
	})

	t.Run("TestLogger", func(t *testing.T) {
		t.Parallel()

		t.Run("With_AddsAttribute", func(t *testing.T) {
			t.Parallel()

			tl := logging.NewTestLogger()
			require.NotNil(t, tl)

			child := tl.With("k", 42)
			child.Info(context.Background(), "msg")

			out := tl.String()
			require.Contains(t, out, "k=42")
		})
	})
}
