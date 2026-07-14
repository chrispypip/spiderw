//go:build fuzz

package iwdbus

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
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
			seq := bytes.SplitSeq(data, []byte{','})
			seq(func(part []byte) bool {
				modes = append(modes, string(part))
				return true
			})
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

func Fuzz_Iwdbus_ParseMode(f *testing.F) {
	f.Add("station")
	f.Add("ap")
	f.Add("ad-hoc")
	f.Add("bogus")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseMode(s)
	})
}

// Fuzz_Iwdbus_EnumParsers exercises the string->enum decoders of externally
// supplied iwd property values. They must never panic on arbitrary input.
func Fuzz_Iwdbus_EnumParsers(f *testing.F) {
	f.Add("station")
	f.Add("connected")
	f.Add("psk")
	f.Add("")
	f.Add("bogus")

	f.Fuzz(func(t *testing.T, s string) {
		// string form
		_, _ = parseNetworkType(s)
		_, _ = parseStationState(s)
		_, _ = parseDeviceMode(s)
		// wrong-type form (exercises the type-assertion failure branch)
		_, _ = parseNetworkType(int64(1))
		_, _ = parseStationState(dbus.MakeVariant(s))
		_, _ = parseDeviceMode(struct{}{})
	})
}

// Fuzz_Iwdbus_ObjectPathParsers exercises the object-path decoders across every
// accepted concrete shape (string, dbus.ObjectPath, dbus.Variant) plus a
// wrong-typed value, ensuring path validation never panics on adversarial input.
func Fuzz_Iwdbus_ObjectPathParsers(f *testing.F) {
	f.Add("/net/connman/iwd/0/3")
	f.Add("/")
	f.Add("")
	f.Add("not-absolute")
	f.Add("/with spaces/and\x00nul")

	f.Fuzz(func(t *testing.T, s string) {
		for _, v := range []interface{}{
			s,
			dbus.ObjectPath(s),
			dbus.MakeVariant(s),
			dbus.MakeVariant(dbus.ObjectPath(s)),
			int64(7), // wrong type
			nil,
		} {
			_, _ = parseNetworkObjectPath("field", v)
			_, _ = parseObjectPath(v)
			_, _ = parseOptionalObjectPath(v)
			_, _ = parseStationObjectPath("field", v)
		}
	})
}

// Fuzz_Iwdbus_ObjectPathListParsers exercises the array-of-object-path decoders
// across the []dbus.ObjectPath and []interface{} forms with adversarial paths.
func Fuzz_Iwdbus_ObjectPathListParsers(f *testing.F) {
	f.Add("/a,/b")
	f.Add("")
	f.Add("not-a-path")

	f.Fuzz(func(t *testing.T, data string) {
		var typed []dbus.ObjectPath
		var iface []interface{}
		if len(data) > 0 {
			for part := range strings.SplitSeq(data, ",") {
				typed = append(typed, dbus.ObjectPath(part))
				iface = append(iface, dbus.ObjectPath(part))
			}
		}
		_, _ = parseObjectPathList(typed)
		_, _ = parseObjectPathList(iface)
		_, _ = parseStationAffinities(typed)
		_, _ = parseStationAffinities(iface)
		_, _ = parseStationAffinities(dbus.MakeVariant(typed))
		// wrong element type + wrong container type
		_, _ = parseObjectPathList([]interface{}{123})
		_, _ = parseStationAffinities("not-an-array")
	})
}

// Fuzz_Iwdbus_ManagedObjectExtractors exercises the managed-object map decoders
// that pull required properties out of an ObjectManager reply, using the fuzz
// input as both the object path and the property value.
func Fuzz_Iwdbus_ManagedObjectExtractors(f *testing.F) {
	f.Add("/net/connman/iwd/0/3", "wlan0")
	f.Add("", "")
	f.Add("bad", "  ")

	f.Fuzz(func(t *testing.T, path string, val string) {
		p := dbus.ObjectPath(path)
		_, _ = bssAddressFromManagedObject(p, map[string]dbus.Variant{"Address": dbus.MakeVariant(val)})
		_, _ = bssAddressFromManagedObject(p, map[string]dbus.Variant{}) // missing
		_, _ = bssAddressFromManagedObject(p, map[string]dbus.Variant{"Address": dbus.MakeVariant(int64(1))})
		_, _ = objectNameFromManagedObject("adapter", p, map[string]dbus.Variant{"Name": dbus.MakeVariant(val)})
		_, _ = objectNameFromManagedObject("device", p, map[string]dbus.Variant{})
		_ = stationNameFromDevice(map[string]map[string]dbus.Variant{
			IwdDeviceIface: {"Name": dbus.MakeVariant(val)},
		})
	})
}

// Fuzz_Iwdbus_ParseDaemonInfo exercises the Daemon.GetInfo payload decoder across
// both accepted map shapes (dbus.Variant-valued and interface{}-valued) plus a
// non-map payload, ensuring malformed daemon metadata never panics.
func Fuzz_Iwdbus_ParseDaemonInfo(f *testing.F) {
	f.Add("1.30", "/var/lib/iwd", true)
	f.Add("", "", false)

	f.Fuzz(func(t *testing.T, version string, stateDir string, netConf bool) {
		_, _ = parseDaemonInfo(map[string]dbus.Variant{
			"Version":                     dbus.MakeVariant(version),
			"StateDirectory":              dbus.MakeVariant(stateDir),
			"NetworkConfigurationEnabled": dbus.MakeVariant(netConf),
		})
		_, _ = parseDaemonInfo(map[string]interface{}{
			"Version":                     version,
			"StateDirectory":              dbus.MakeVariant(stateDir),
			"NetworkConfigurationEnabled": netConf,
		})
		// wrong field types + non-map payload
		_, _ = parseDaemonInfo(map[string]dbus.Variant{"Version": dbus.MakeVariant(int64(1))})
		_, _ = parseDaemonInfo(version)
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

// Fuzz_Iwdbus_AccessPointOptionalParsers exercises the decoders for the AccessPoint
// properties iwd only exposes while the AP is running (Name, Frequency, the
// ciphers). Each accepts a nil, a concrete value, or a dbus.Variant wrapping
// either, so all three shapes plus a wrong-typed value must stay panic-free.
func Fuzz_Iwdbus_AccessPointOptionalParsers(f *testing.F) {
	f.Add("MyAP")
	f.Add("")
	f.Add("CCMP")

	f.Fuzz(func(t *testing.T, s string) {
		// nil (the absent-property shape)
		_, _ = parseOptionalAccessPointString("Name", nil)
		_, _ = parseAccessPointFrequency(nil)
		_, _ = parseAccessPointCiphers(nil)

		// concrete form
		_, _ = parseOptionalAccessPointString("Name", s)
		_, _ = parseAccessPointCiphers([]string{s})

		// variant-wrapped form (recurses through the unwrap branch)
		_, _ = parseOptionalAccessPointString("GroupCipher", dbus.MakeVariant(s))
		_, _ = parseAccessPointFrequency(dbus.MakeVariant(uint32(len(s))))
		_, _ = parseAccessPointCiphers(dbus.MakeVariant([]string{s}))

		// wrong-typed form (exercises the type-assertion failure branch)
		_, _ = parseOptionalAccessPointString("Name", int64(1))
		_, _ = parseAccessPointFrequency(s)
		_, _ = parseAccessPointCiphers(s)
	})
}

// Fuzz_Iwdbus_ParseAccessPointOrderedNetwork exercises the neighbor-dict decoder
// for AccessPoint.GetOrderedNetworks. Every key is daemon-supplied and each may
// be absent or of any D-Bus type, so the parser must never panic - it may only
// return an error. The Type key (which carries the security) is the interesting
// one: an unrecognized string is tolerated as unknown, while a non-string is an
// error, and neither may crash.
func Fuzz_Iwdbus_ParseAccessPointOrderedNetwork(f *testing.F) {
	f.Add("OpenNet", int16(-6000), "open")
	f.Add("SecuredNet", int16(-7200), "psk")
	f.Add("", int16(0), "")
	f.Add("MysteryNet", int16(32767), "wpa9000")

	f.Fuzz(func(t *testing.T, name string, signal int16, netType string) {
		// Well-typed entry: arbitrary values, correct D-Bus types.
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{
			"Name":           dbus.MakeVariant(name),
			"SignalStrength": dbus.MakeVariant(signal),
			"Type":           dbus.MakeVariant(netType),
		})

		// Missing keys: iwd may omit any of them.
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{})
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{
			"Name": dbus.MakeVariant(name),
		})

		// Wrong-typed keys, one at a time, exercising each failure branch.
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{
			"Name": dbus.MakeVariant(int64(len(name))),
		})
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{
			"SignalStrength": dbus.MakeVariant(netType),
		})
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{
			"Type": dbus.MakeVariant(signal),
		})

		// An unexpected extra key must simply be ignored.
		_, _ = parseAccessPointOrderedNetwork(map[string]dbus.Variant{
			"Name":     dbus.MakeVariant(name),
			"Security": dbus.MakeVariant(netType),
			netType:    dbus.MakeVariant(name),
		})
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
