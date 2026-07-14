//go:build regression

package iwdbus

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/stretchr/testify/require"
)

func TestRegression_Iwdbus_IntrospectedObject_CloseDuringEmit(t *testing.T) {
	// Intentionally not parallel: this test stresses goroutine interleavings
	// and relies on global scheduling behavior.
	const iterations = 50

	for iter := range iterations {
		t.Run("iter_"+itoa(iter), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			intro := &IntrospectedObject{
				Conn:             nil, // hermetic: no real DBus
				BusName:          "test.bus",
				Path:             dbus.ObjectPath("/test/path"),
				ctx:              ctx,
				cancel:           cancel,
				ifaces:           map[string]*introspect.Interface{},
				sigCh:            make(chan *dbus.Signal, dbusSignalCapacity),
				handlersExact:    map[string][]signalHandler{},
				handlersWildcard: map[string][]signalHandler{},
			}

			const handlerCount = 8
			var calls atomic.Int64

			for range handlerCount {
				err := intro.RegisterSignalHandler("test.iface", "TestSignal", func(*dbus.Signal) {
					calls.Add(1)
				})
				require.NoError(t, err)
			}

			stop := make(chan struct{})
			go func() {
				sig := &dbus.Signal{
					Name: "test.iface.TestSignal",
					Path: intro.Path,
					Body: []interface{}{"x"},
				}
				for {
					select {
					case <-stop:
						return
					default:
						select {
						case intro.sigCh <- sig:
						default:
							time.Sleep(50 * time.Microsecond)
						}
					}
				}
			}()

			// Ensure dispatcher is active
			deadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(deadline) {
				if calls.Load() > 0 {
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
			if calls.Load() == 0 {
				close(stop)
				t.Fatalf("dispatcher did not process any signals")
			}

			// Close while signals are actively being emitted.
			_ = intro.Close()

			// Stop emitter after Close returns.
			close(stop)

			// Assert no handler invocations after Close returns.
			c1 := calls.Load()
			time.Sleep(50 * time.Millisecond)
			c2 := calls.Load()

			if c2 != c1 {
				t.Fatalf("handler calls continued after Close returned: before=%d after=%d", c1, c2)
			}

			// Close should be idempotent and safe.
			_ = intro.Close()
		})
	}
}

// tiny helper to avoid fmt.Sprintf allocations in tight loops
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

// The tests below guard bugs found on real hardware (a Raspberry Pi running iwd
// 3.12) that every mock-backed test missed. Each one shipped green because the mock
// was more forgiving than the daemon; each is recorded here so the specific wire
// behavior can never silently regress. They intentionally duplicate unit coverage —
// the point is provenance, not novelty.

// TestRegression_Iwdbus_StoppedAccessPointOmitsOptionalProperties: iwd documents
// AccessPoint.Scanning as always present. It is not. A stopped AP reports only
// Started, so a strict read of Scanning failed on hardware with
// "property=Scanning: missing required property" while the mock (which always sent
// it) passed.
func TestRegression_Iwdbus_StoppedAccessPointOmitsOptionalProperties(t *testing.T) {
	a := &AccessPoint{call: &fakeCaller{
		getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
			return map[string]dbus.Variant{"Started": dbus.MakeVariant(false)}, nil
		},
	}}

	props, err := a.GetProperties(context.Background())
	require.NoError(t, err, "a stopped AP reports only Started; that must not be an error")
	require.False(t, props.Started)
	require.False(t, props.Scanning, "absent Scanning collapses to false")
	require.Nil(t, props.Name)
	require.Nil(t, props.Frequency)
}

// TestRegression_Iwdbus_OrderedNetworksSecurityKeyIsType: iwd's documentation calls
// the security key in GetOrderedNetworks "Security". On the wire it is "Type" —
// confirmed by calling the method with gdbus. Reading "Security" yielded an unknown
// security for every neighbor.
func TestRegression_Iwdbus_OrderedNetworksSecurityKeyIsType(t *testing.T) {
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{
				[]map[string]dbus.Variant{{
					"Name":           dbus.MakeVariant("Neighbor"),
					"SignalStrength": dbus.MakeVariant(int16(-4600)),
					"Type":           dbus.MakeVariant("psk"),
				}},
			}, nil
		},
	}}

	nets, err := a.GetOrderedNetworks(context.Background())
	require.NoError(t, err)
	require.Len(t, nets, 1)
	require.Equal(t, NetworkTypePSK, nets[0].Type,
		`the security lives in the "Type" key, not "Security"`)
}

// TestRegression_Iwdbus_OrderedNetworksToleratesUnclassifiedNeighbor: one neighbor
// whose security iwd could not classify failed the entire `access-point networks`
// listing on hardware. An unrecognized Type string must degrade to unknown, not
// abort the reply — while a Type of the wrong D-Bus type is still an error.
func TestRegression_Iwdbus_OrderedNetworksToleratesUnclassifiedNeighbor(t *testing.T) {
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{
				[]map[string]dbus.Variant{
					{"Name": dbus.MakeVariant("Good"), "Type": dbus.MakeVariant("psk")},
					{"Name": dbus.MakeVariant("Weird"), "Type": dbus.MakeVariant("wpa9000")},
				},
			}, nil
		},
	}}

	nets, err := a.GetOrderedNetworks(context.Background())
	require.NoError(t, err, "one unclassifiable neighbor must not fail the whole list")
	require.Len(t, nets, 2)
	require.Equal(t, NetworkTypePSK, nets[0].Type)
	require.Equal(t, NetworkTypeUnknown, nets[1].Type)
}

// TestRegression_Iwdbus_DisconnectInvalidatesRatherThanSendingNullPath: iwd does not
// report "no longer connected" by sending the null path "/" in Changed — it lists
// the property in Invalidated and sends no value. Subscriptions that read only
// Changed were silent on every disconnect on hardware, while the mock (which sent
// "/") looked correct.
func TestRegression_Iwdbus_DisconnectInvalidatesRatherThanSendingNullPath(t *testing.T) {
	for _, tc := range []struct {
		name string
		sub  func(*Station, chan *string) error
		prop string
	}{
		{"ConnectedNetwork", func(s *Station, got chan *string) error {
			_, err := s.SubscribeConnectedNetworkChanged(context.Background(), func(p *string) { got <- p })
			return err
		}, "ConnectedNetwork"},
		{"ConnectedAccessPoint", func(s *Station, got chan *string) error {
			_, err := s.SubscribeConnectedAccessPointChanged(context.Background(), func(p *string) { got <- p })
			return err
		}, "ConnectedAccessPoint"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeSignalSource(t)
			station := &Station{signals: fake}

			got := make(chan *string, 1)
			require.NoError(t, tc.sub(station, got))

			// The disconnect as iwd actually sends it: a State change, and the path
			// property invalidated with no value at all.
			fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface,
				map[string]dbus.Variant{"State": dbus.MakeVariant("disconnected")},
				[]string{tc.prop})

			select {
			case p := <-got:
				require.Nil(t, p, "an invalidated path means gone")
			case <-time.After(time.Second):
				t.Fatalf("%s subscription never fired on an invalidation", tc.prop)
			}
		})
	}
}

// TestRegression_Iwdbus_ForgetInvalidatesKnownNetwork: the same invalidation
// behavior for Network.KnownNetwork. Forgetting a network on hardware produced no
// KnownNetwork value, so a Changed-only subscription never saw the forget.
func TestRegression_Iwdbus_ForgetInvalidatesKnownNetwork(t *testing.T) {
	fake := newFakeSignalSource(t)
	n := &Network{signals: fake}

	got := make(chan *string, 1)
	_, err := n.SubscribeKnownNetworkChanged(context.Background(), func(p *string) { got <- p })
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdNetworkIface,
		map[string]dbus.Variant{}, []string{"KnownNetwork"})

	select {
	case p := <-got:
		require.Nil(t, p, "an invalidated KnownNetwork means forgotten")
	case <-time.After(time.Second):
		t.Fatal("KnownNetwork subscription never fired on an invalidation")
	}
}

// TestRegression_Iwdbus_AbsentOptionalPropertyWording pins the contract between how
// iwd words "that optional property has no value" and the matcher that decides
// whether a getter failure means "absent" or "broken".
//
// This is load-bearing and invisible: every optional property (Adapter Model/Vendor,
// Network KnownNetwork, KnownNetwork LastConnectedTime, the AccessPoint fields on a
// stopped AP, the Station fields when disconnected) reads through it. The mock got
// this wrong for Network, KnownNetwork and BSS — it said `unknown property "X"`,
// which the matcher does not recognize — so reading an unprovisioned network's
// KnownNetwork failed against the mock while it would have succeeded against iwd.
// Nobody had read those properties in the absent case, so it sat there.
//
// If the regex is ever "tidied", optional properties break everywhere at once.
func TestRegression_Iwdbus_AbsentOptionalPropertyWording(t *testing.T) {
	// The wordings iwd and ELL actually emit, across versions and casings.
	for _, msg := range []string{
		"Getting property value failed",
		"getting property value failed",
		"GetProperty failed: unknown property",
		"getproperty failed: unknown property",
	} {
		require.True(t, isUnknownPropertyError(fmt.Errorf("%s", msg)),
			"iwd says %q when an optional property is absent; the matcher must recognize it", msg)
	}

	// And it must not swallow a genuine failure as "just absent".
	for _, msg := range []string{
		"Operation not available",
		"Operation not permitted",
		"unknown property",
		"Not connected",
		"",
	} {
		require.False(t, isUnknownPropertyError(fmt.Errorf("%s", msg)),
			"a real failure (%q) must not be mistaken for an absent property", msg)
	}
}

// TestRegression_Iwdbus_StoppedAccessPointRejectsScan: an access point that is not
// running has no radio configured to survey with, so iwd rejects both Scan and
// GetOrderedNetworks with NotAvailable ("Operation not available", confirmed on
// hardware). The mock used to run the scan and hand back seeded results.
func TestRegression_Iwdbus_StoppedAccessPointRejectsScan(t *testing.T) {
	notAvailable := dbus.Error{
		Name: IwdErrorNotAvailable,
		Body: []interface{}{"Operation not available"},
	}

	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, notAvailable
		},
	}}

	require.ErrorIs(t, a.Scan(context.Background()), ErrNotAvailable)

	_, err := a.GetOrderedNetworks(context.Background())
	require.ErrorIs(t, err, ErrNotAvailable)
}
