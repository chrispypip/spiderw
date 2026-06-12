//go:build bench

package iwdbus

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
)

// Benchmarks for internal/iwdbus focus on dispatcher mechanics and handler fan-out.
// They intentionally avoid real DBus connections and measure only in-process costs.

func Benchmark_Iwdbus_RegisterSignalHandler(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = i.RegisterSignalHandler("iface", "member", func(*dbus.Signal) {})
	}
}

func Benchmark_Iwdbus_Dispatch_SingleExactHandler(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	_ = i.RegisterSignalHandler("iface", "member", func(*dbus.Signal) {})
	sig := benchSignal()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		select {
		case i.sigCh <- sig:
		default:
		}
	}
}

func Benchmark_Iwdbus_Dispatch_SingleWildcardHandler(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	_ = i.RegisterSignalHandler("*", "*", func(*dbus.Signal) {})
	sig := benchSignal()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		select {
		case i.sigCh <- sig:
		default:
		}
	}
}

func Benchmark_Iwdbus_Dispatch_ManyHandlers(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	for range 32 {
		_ = i.RegisterSignalHandler("iface", "member", func(*dbus.Signal) {})
	}
	sig := benchSignal()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		select {
		case i.sigCh <- sig:
		default:
		}
	}
}

func Benchmark_Iwdbus_Dispatch_MixedExactAndWildcard(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	// Exact handlers
	for range 16 {
		_ = i.RegisterSignalHandler("iface", "member", func(*dbus.Signal) {})
	}
	// Wildcard handlers
	for range 16 {
		_ = i.RegisterSignalHandler("*", "*", func(*dbus.Signal) {})
	}

	sig := benchSignal()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		select {
		case i.sigCh <- sig:
		default:
		}
	}
}

func Benchmark_Iwdbus_Dispatch_SlowHandlerIsolation(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	// One slow handler
	_ = i.RegisterSignalHandler("iface", "member", func(*dbus.Signal) {
		// simulate slow work without sleeping
		for n := range 1000 {
			_ = n * n
		}
	})

	// Many fast handlers
	for range 31 {
		_ = i.RegisterSignalHandler("iface", "member", func(*dbus.Signal) {})
	}

	sig := benchSignal()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		select {
		case i.sigCh <- sig:
		default:
		}
	}
}

func Benchmark_Iwdbus_Dispatch_StartupShutdown(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		i := newBenchIntrospectedObject(b)
		_ = i.Close()
	}
}

func Benchmark_Iwdbus_Dispatcher_Parallel(b *testing.B) {
	i := newBenchIntrospectedObject(b)
	defer i.Close()

	sig := benchSignal()

	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			select {
			case i.sigCh <- sig:
			default:
			}
		}
	})
}

// newBenchIntrospectedObject constructs an IntrospectedObject equivalent to
// NewIntrospectedObject but without touching the system bus.
func newBenchIntrospectedObject(b *testing.B) *IntrospectedObject {
	b.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	i := &IntrospectedObject{
		Conn:             nil,
		BusName:          "bench",
		Path:             dbus.ObjectPath("/bench"),
		ctx:              ctx,
		cancel:           cancel,
		sigCh:            make(chan *dbus.Signal, dbusSignalCapacity),
		handlersExact:    map[string][]signalHandler{},
		handlersWildcard: map[string][]signalHandler{},
	}

	i.startDispatcher()
	return i
}

func benchSignal() *dbus.Signal {
	return &dbus.Signal{
		Name: "iface.member",
		Path: dbus.ObjectPath("/bench"),
		Body: []interface{}{int64(1)},
	}
}
