//go:build bench

package iwdbus

import (
	"context"
	"fmt"
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

// Data-path benchmarks: these measure the in-process CPU/allocation cost of the
// parse and resolution work that runs per status-read (GetProperties, the
// documented batched "one GetAll" optimization) and per enumeration/resolution
// (object-path list decoding, ObjectTree lookups). They avoid real DBus and
// scale their inputs so a regression in the parse/lookup logic is visible.

func benchObjectPaths(n int) []dbus.ObjectPath {
	paths := make([]dbus.ObjectPath, n)
	for i := range paths {
		paths[i] = dbus.ObjectPath(fmt.Sprintf("/net/connman/iwd/0/3/ssid_psk/%012x", i))
	}
	return paths
}

func Benchmark_Iwdbus_Station_GetProperties(b *testing.B) {
	affinities := benchObjectPaths(8)
	props := map[string]dbus.Variant{
		"State":                dbus.MakeVariant("connected"),
		"Scanning":             dbus.MakeVariant(false),
		"ConnectedNetwork":     dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/0/3/ssid_psk")),
		"ConnectedAccessPoint": dbus.MakeVariant(affinities[0]),
		"Affinities":           dbus.MakeVariant(affinities),
	}
	s := &Station{call: &fakeCaller{
		getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
			return props, nil
		},
	}}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.GetProperties(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Iwdbus_ParseObjectPathList(b *testing.B) {
	// A busy network's ExtendedServiceSet / a post-scan result set.
	raw := benchObjectPaths(64)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseObjectPathList(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Iwdbus_ParseStationAffinities(b *testing.B) {
	raw := benchObjectPaths(32)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := parseStationAffinities(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Iwdbus_AccessPoint_GetOrderedNetworks(b *testing.B) {
	// A busy AP scan result: many neighbor dicts (aa{sv}) to parse per call.
	entries := make([]map[string]dbus.Variant, 64)
	for i := range entries {
		entries[i] = map[string]dbus.Variant{
			"Name":           dbus.MakeVariant(fmt.Sprintf("Neighbor-%d", i)),
			"SignalStrength": dbus.MakeVariant(int16(-4000 - i*50)),
			"Type":           dbus.MakeVariant("psk"),
		}
	}
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{entries}, nil
		},
	}}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.GetOrderedNetworks(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Iwdbus_ObjectTree_Lookups(b *testing.B) {
	// A tree the size of a busy multi-radio host after a scan: many network and
	// BSS objects, resolved per-ref during Properties() bundle enrichment.
	const n = 128
	objects := make(map[dbus.ObjectPath]map[string]map[string]dbus.Variant, 2*n)
	netPaths := make([]string, n)
	bssPaths := make([]string, n)
	for i := range n {
		np := fmt.Sprintf("/net/connman/iwd/0/3/ssid_%04x", i)
		bp := fmt.Sprintf("%s/%012x", np, i)
		netPaths[i] = np
		bssPaths[i] = bp
		objects[dbus.ObjectPath(np)] = map[string]map[string]dbus.Variant{
			IwdNetworkIface: {"Name": dbus.MakeVariant(fmt.Sprintf("SSID-%d", i))},
		}
		objects[dbus.ObjectPath(bp)] = map[string]map[string]dbus.Variant{
			IwdBasicServiceSetIface: {"Address": dbus.MakeVariant(fmt.Sprintf("%012x", i))},
		}
	}
	tree := &ObjectTree{objects: objects}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		idx := i % n
		_, _ = tree.NetworkName(netPaths[idx])
		_, _ = tree.BSSAddress(bssPaths[idx])
	}
}
