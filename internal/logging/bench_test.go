//go:build bench

package logging

import (
	"context"
	"fmt"
	"testing"
)

// Benchmarks for internal/logging focus on the cost of the logging abstraction
// itself (With() chaining, logging calls, allocations). They intentionally avoid
// correctness assertions and external I/O.

var benchCtx = context.Background()

func Benchmark_Logging_Debug_NoFields(b *testing.B) {
	var l Logger = NewTestLogger()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		l.Debug(benchCtx, "msg")
	}
}

func Benchmark_Logging_Debug_WithFewFields(b *testing.B) {
	l := NewTestLogger().With(
		"a", 1,
		"b", "two",
		"c", true,
	)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		l.Debug(benchCtx, "msg")
	}
}

func Benchmark_Logging_Debug_WithManyFields(b *testing.B) {
	fields := make([]any, 0, 40)
	for i := range 20 {
		fields = append(fields, "k", i)
	}

	l := NewTestLogger().With(fields...)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		l.Debug(benchCtx, "msg")
	}
}

func Benchmark_Logging_Debug_Parallel(b *testing.B) {
	l := Logger(NewTestLogger())
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Debug(benchCtx, "msg")
		}
	})
}

func Benchmark_Logging_WithChainDepth(b *testing.B) {
	for _, depth := range []int{1, 5, 10, 25, 50} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			base := Logger(NewTestLogger())
			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				l := base
				for d := range depth {
					l = l.With("k", d)
				}
			}
		})
	}
}

func Benchmark_Logging_WithOnly(b *testing.B) {
	l := NewTestLogger()

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		_ = l.With("k", i)
	}
}
