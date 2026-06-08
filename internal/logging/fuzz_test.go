//go:build fuzz

package logging

import (
	"bytes"
	"context"
	"testing"
)

const maxFuzzInput = 256

// Fuzzing goals for internal/logging:
// - logging must never panic regardless of input
// - With() chaining must be safe under arbitrary key/value shapes
//
// These fuzz tests are survivability-oriented: no correctness assertions,
// only "must not crash or deadlock" invariants.

func Fuzz_Logging_WithSingle(f *testing.F) {
	f.Add([]byte("key=value"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFuzzInput {
			data = data[:maxFuzzInput]
		}

		// Use the concrete TestLogger only for construction; treat it as Logger
		// thereafter since With() returns Logger.
		var l Logger = NewTestLogger()
		kv := decodeKeyValues(data)

		// Chaining With() should never panic, even with garbage.
		l = l.With(kv...)
		ctx := context.Background()
		if len(data) > 0 && data[0]%2 == 0 {
			ctx = context.WithValue(ctx, struct{}{}, data)
		}
		l.Debug(ctx, "msg")
	})
}

func Fuzz_Logging_Message(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte{0xff, 0xfe, 0xfd})

	f.Fuzz(func(t *testing.T, msg []byte) {
		var l Logger = NewTestLogger()
		l.Info(context.Background(), string(msg))
		l.Warn(context.Background(), string(msg))
		l.Error(context.Background(), string(msg))
	})
}

func Fuzz_Logging_WithDeepChain(f *testing.F) {
	f.Add([]byte("deep"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var l Logger = NewTestLogger()

		// Build a deep With() chain based on input size.
		depth := min(len(data), 100)

		for i := range depth {
			key := string([]byte{byte(i % 256)})
			l = l.With(key, i)
		}

		// Exercise logging after deep chaining.
		l.Info(context.Background(), string(bytes.Repeat(data, 1)))
	})
}

// decodeKeyValues turns arbitrary bytes into a slice of key/value values.
// It intentionally produces odd-length slices, nils, and strange types
// to exercise defensive handling in With().
func decodeKeyValues(b []byte) []any {
	out := make([]any, 0, len(b))

	for i := range len(b) {
		switch b[i] % 5 {
		case 0:
			out = append(out, string(b))
		case 1:
			out = append(out, b)
		case 2:
			out = append(out, int(b[i]))
		case 3:
			out = append(out, nil)
		case 4:
			out = append(out, map[string]any{"k": b})
		}
	}

	return out
}
