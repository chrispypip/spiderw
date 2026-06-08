//go:build fuzz

package iwdbus

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/godbus/dbus/v5"
)

// Fuzzing goals for iwdbus:
// - never panic on malformed / adversarial signals
// - never deadlock on handler registration + dispatch
// - tolerate handler panics (safeInvoke)
// - keep parsing helpers panic-free

type fuzzErr struct{ s string }

func (e *fuzzErr) Error() string { return e.s }

func Fuzz_Iwdbus_DispatchSignal(f *testing.F) {
	f.Add("org.freedesktop.DBus.Properties.PropertiesChanged", "/net/connman/iwd", []byte("hello"))
	f.Add("iface.member", "/fuzz", []byte{0, 1, 2, 3, 4, 5})
	f.Add("bad", "not-a-path", []byte{})

	f.Fuzz(func(t *testing.T, name string, path string, payload []byte) {
		i := newFuzzIntrospectedObject(t)
		sig := &dbus.Signal{
			Name: name,
			Path: dbus.ObjectPath(path),
			Body: bodyFromBytes(payload),
		}

		if len(payload)%2 == 0 {
			registerFuzzHandlers(i)
			emitNonBlocking(i, sig)
		} else {
			emitNonBlocking(i, sig)
			registerFuzzHandlers(i)
		}

		_ = i.Close()
		_ = i.Close()
	})
}

func Fuzz_Iwdbus_RegisterHandlers(f *testing.F) {
	f.Add("*", "*", "iface.member")
	f.Add("org.freedesktop.DBus.Properties", "PropertiesChanged", "org.freedesktop.DBus.Properties.PropertiesChanged")
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, iface string, member string, signalName string) {
		i := newFuzzIntrospectedObject(t)

		// Register arbitrary patterns; errors are acceptable, panics are not.
		_ = i.RegisterSignalHandler(iface, member, func(*dbus.Signal) {})
		_ = i.RegisterSignalHandler("*", member, func(*dbus.Signal) {})
		_ = i.RegisterSignalHandler(iface, "*", func(*dbus.Signal) {})

		sig := &dbus.Signal{Name: signalName, Path: dbus.ObjectPath("/fuzz")}
		emitNonBlocking(i, sig)
		_ = i.Close()
	})
}

func Fuzz_Iwdbus_SplitSignalName(f *testing.F) {
	f.Add("org.freedesktop.DBus.Properties.PropertiesChanged")
	f.Add("iface.member")
	f.Add("no-dot")
	f.Add("..")
	f.Add("")
	f.Add("0")

	f.Fuzz(func(t *testing.T, name string) {
		iface, member := splitSignalName(name)

		// Invariants must not panic and outputs must be deterministic strings
		_ = iface
		_ = member

		if name == "" && (iface != "" || member != "") {
			t.Fatalf("unexpected output for empty input: iface=%q member=%q", iface, member)
		}
		_ = member
	})
}

func Fuzz_Iwdbus_ParseOptionalString(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, b []byte) {
		// Exercise multiple accepted/invalid shapes.
		_, _ = parseOptionalString(nil)
		_, _ = parseOptionalString(string(b))
		_, _ = parseOptionalString(dbus.MakeVariant(string(b)))
		_, _ = parseOptionalString(dbus.MakeVariant(int64(123)))
		_, _ = parseOptionalString(int(7))
	})
}

func Fuzz_Iwdbus_ParseSupportedModes(f *testing.F) {
	f.Add([]byte("station,ap"))
	f.Add([]byte("ad-hoc"))
	f.Add([]byte("bogus"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode fuzz input into []string safely
		var modes []string
		if len(data) > 0 {
			parts := bytes.Split(data, []byte{','})
			for _, p := range parts {
				modes = append(modes, string(p))
			}
		}

		// []string path
		_, _ = parseSupportedModes(modes)

		// []interface{} path
		raw := make([]interface{}, 0, len(modes))
		for _, s := range modes {
			raw = append(raw, s)
		}
		_, _ = parseSupportedModes(raw)

		// Mixed invalid types
		if len(raw) > 0 {
			raw2 := append([]interface{}{}, raw...)
			raw2[0] = 123
			_, _ = parseSupportedModes(raw2)
		}
	})
}

func Fuzz_Iwdbus_ParseAdapterMode(f *testing.F) {
	f.Add("station")
	f.Add("ap")
	f.Add("ad-hoc")
	f.Add("bogus")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseAdapterMode(s)
	})
}

func Fuzz_Iwdbus_IsUnknownPropertyError(f *testing.F) {
	f.Add([]byte("GetProperty failed: unknown property"))
	f.Add([]byte("some other error"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = isUnknownPropertyError(&fuzzErr{s: string(data)})
	})
}

func Fuzz_Iwdbus_ParseIntrospectionChildNames(f *testing.F) {
	// A few representative introspection payloads.
	f.Add([]byte(`<node></node>`))
	f.Add([]byte(`<node><node name="phy0"/><node name="phy1"/></node>`))
	f.Add([]byte(`<node><node name="  phy0  "/><node  name="nested"><node name="child"/></node><node>`))
	f.Add([]byte(`<not-xml`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic on arbitrary input.
		_, _ = parseIntrospectionChildNames(string(data))
	})
}

func newFuzzIntrospectedObject(t *testing.T) *IntrospectedObject {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	i := &IntrospectedObject{
		Conn:             nil, // important: no real DBus access during fuzz
		BusName:          "fuzz",
		Path:             dbus.ObjectPath("/fuzz"),
		ctx:              ctx,
		cancel:           cancel,
		sigCh:            make(chan *dbus.Signal, dbusSignalCapacity),
		handlersExact:    map[string][]signalHandler{},
		handlersWildcard: map[string][]signalHandler{},
	}

	// Start dispatcher like NewIntrospectedObject would.
	i.startDispatcher()
	return i
}

func emitNonBlocking(i *IntrospectedObject, sig *dbus.Signal) {
	select {
	case i.sigCh <- sig:
	default:
		// Drop if channel is full to avoid fuzz deadlocks.
	}
}

func bodyFromBytes(b []byte) []interface{} {
	// Produce a mix of values; dispatcher does not interpret Body, but
	// handlers might, and we want to exercise safeInvoke + robustness.
	out := make([]interface{}, 0, 8)

	// Always include the raw bytes.
	out = append(out, append([]byte(nil), b...))

	// Add some derived primitives.
	if len(b) > 0 {
		out = append(out, b[0]%2 == 0)
		out = append(out, int64(b[0]))
		out = append(out, string(b))
	}

	// Add a dbus.ObjectPath-ish value sometimes.
	if len(b) > 1 {
		// Keep it reasonably path-like, but allow weird chars.
		out = append(out, dbus.ObjectPath("/"+string(b)))
	}

	// Add a dbus.Variant sometimes.
	if len(b) >= 8 {
		var buf [8]byte
		copy(buf[:], b)
		n := int64(binary.LittleEndian.Uint16(buf[:]))
		out = append(out, dbus.MakeVariant(n))
	}

	return out
}

func registerFuzzHandlers(i *IntrospectedObject) {
	// Safe no-op handler.
	_ = i.RegisterSignalHandler("*", "*", func(signal *dbus.Signal) {})

	// Handler that may panic if body shapes/types are unexpected.
	// safeInvoke must contain this.
	_ = i.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
		if sig == nil {
			return
		}
		if len(sig.Body) > 0 {
			_ = sig.Body[0].(string) // may panic
		}
	})

	// Handler that may panic based on the signal name; safeInvoke must contain.
	_ = i.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
		if sig == nil {
			return
		}
		// Deliberate panic trigger for some inputs.
		if bytes.Contains([]byte(sig.Name), []byte("panic")) {
			panic("fuzz panic")
		}
	})
}
