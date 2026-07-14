//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// This file mirrors device_test.go's grouped t.Run subtree structure.

func TestStation_Core(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("NewStation", func(t *testing.T) {
		tests := []struct {
			name    string
			in      stationRaw
			wantNil bool
		}{
			{name: "nil", in: nil, wantNil: true},
			{name: "non-nil", in: &fakeIwdbusStation{}, wantNil: false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				s := NewStation(tc.in)
				if tc.wantNil {
					require.Nil(t, s)
					return
				}
				require.NotNil(t, s)
			})
		}
	})

	t.Run("State", func(t *testing.T) {

		t.Run("DBusErrorMapping", func(t *testing.T) {
			tests := []struct {
				name     string
				dbusErr  error
				wantKind Kind
			}{
				{name: "connection", dbusErr: iwdbus.ErrDBusConnection, wantKind: KindUnavailable},
				{name: "method", dbusErr: iwdbus.ErrDBusMethod, wantKind: KindUnavailable},
				{name: "property", dbusErr: iwdbus.ErrDBusProperty, wantKind: KindUnavailable},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					f := &fakeIwdbusStation{}
					f.state.Store(iwdbus.StationStateConnected)
					f.setErr(tc.dbusErr)
					_, err := NewStation(f).State(ctx)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, ResourceStation, ce.Resource)
					require.ErrorIs(t, err, tc.dbusErr)
				})
			}
		})

		t.Run("UnknownIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.state.Store(iwdbus.StationState("bogus"))
			_, err := NewStation(f).State(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			state, err := newTestStation(t).State(ctx)
			require.NoError(t, err)
			require.Equal(t, StationStateConnected, state)
		})
	})

	t.Run("Scanning", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).Scanning(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.scanning.Store(true)
			v, err := NewStation(f).Scanning(ctx)
			require.NoError(t, err)
			require.True(t, v)
		})
	})

	t.Run("ConnectedNetwork", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).ConnectedNetwork(ctx)
			require.Error(t, err)
		})

		t.Run("NilWhenDisconnected", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			// connectedNetwork left nil
			got, err := NewStation(f).ConnectedNetwork(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})

		t.Run("Success", func(t *testing.T) {
			got, err := newTestStation(t).ConnectedNetwork(ctx)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, "/net/connman/iwd/phy0/wlan0/net0", *got)
		})
	})

	t.Run("ConnectedAccessPoint", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).ConnectedAccessPoint(ctx)
			require.Error(t, err)
		})

		t.Run("NilWhenAbsent", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			got, err := NewStation(f).ConnectedAccessPoint(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			ap := "/net/connman/iwd/phy0/wlan0/abc123"
			f.connectedAccessPoint.Store(&ap)
			got, err := NewStation(f).ConnectedAccessPoint(ctx)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, ap, *got)
		})
	})

	t.Run("Affinities", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).Affinities(ctx)
			require.Error(t, err)
		})

		t.Run("NilWhenAbsent", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			got, err := NewStation(f).Affinities(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			affinities := []string{"/net/connman/iwd/phy0/wlan0/aaa"}
			f.affinities.Store(&affinities)
			got, err := NewStation(f).Affinities(ctx)
			require.NoError(t, err)
			require.Equal(t, affinities, got)
		})
	})

	t.Run("Properties", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).Properties(ctx)
			require.Error(t, err)
		})

		t.Run("UnknownStateIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.state.Store(iwdbus.StationState("nope"))
			_, err := NewStation(f).Properties(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
		})

		t.Run("Disconnected", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.state.Store(iwdbus.StationStateDisconnected)
			// connectedNetwork left nil
			props, err := NewStation(f).Properties(ctx)
			require.NoError(t, err)
			require.Equal(t, StationStateDisconnected, props.State)
			require.Nil(t, props.ConnectedNetwork)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.state.Store(iwdbus.StationStateConnected)
			net := "/net/connman/iwd/phy0/wlan0/net0"
			f.connectedNetwork.Store(&net)
			ap := "/net/connman/iwd/phy0/wlan0/abc123"
			f.connectedAccessPoint.Store(&ap)
			affinities := []string{ap}
			f.affinities.Store(&affinities)

			props, err := NewStation(f).Properties(ctx)
			require.NoError(t, err)
			require.Equal(t, StationStateConnected, props.State)
			require.False(t, props.Scanning)
			require.NotNil(t, props.ConnectedNetwork)
			require.Equal(t, net, *props.ConnectedNetwork)
			require.NotNil(t, props.ConnectedAccessPoint)
			require.Equal(t, ap, *props.ConnectedAccessPoint)
			require.Equal(t, affinities, props.Affinities)
		})
	})

	t.Run("Scan", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			err := NewStation(f).Scan(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			require.NoError(t, NewStation(f).Scan(ctx))
			require.True(t, f.scanCalled.Load())
		})
	})

	t.Run("OrderedNetworks", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).OrderedNetworks(ctx)
			require.Error(t, err)
		})

		t.Run("InvalidPathIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			nets := []iwdbus.OrderedNetwork{{Network: "not/abs", SignalStrength: -6000}}
			f.orderedNetworks.Store(&nets)
			_, err := NewStation(f).OrderedNetworks(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			nets := []iwdbus.OrderedNetwork{
				{Network: "/net/connman/iwd/phy0/wlan0/net0", SignalStrength: -6000},
				{Network: "  /net/connman/iwd/phy0/wlan0/net1  ", SignalStrength: -7200},
			}
			f.orderedNetworks.Store(&nets)
			got, err := NewStation(f).OrderedNetworks(ctx)
			require.NoError(t, err)
			require.Equal(t, []OrderedNetwork{
				{Network: "/net/connman/iwd/phy0/wlan0/net0", SignalStrength: -6000},
				{Network: "/net/connman/iwd/phy0/wlan0/net1", SignalStrength: -7200},
			}, got)
		})
	})

	t.Run("SetAffinities", func(t *testing.T) {

		t.Run("InvalidPathIsInvalidArgument", func(t *testing.T) {
			tests := []string{"", "  ", "relative/path"}
			for _, bad := range tests {
				f := &fakeIwdbusStation{}
				err := NewStation(f).SetAffinities(ctx, []string{bad})
				require.Error(t, err)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, KindInvalidArgument, ce.Kind)
				require.Equal(t, ResourceStation, ce.Resource)
				require.Nil(t, f.setAffinitiesArg.Load(), "backend must not be called for invalid input")
			}
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusProperty)
			err := NewStation(f).SetAffinities(ctx, []string{"/net/connman/iwd/phy0/wlan0/aaa"})
			require.Error(t, err)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			paths := []string{"/net/connman/iwd/phy0/wlan0/aaa", "/net/connman/iwd/phy0/wlan0/bbb"}
			require.NoError(t, NewStation(f).SetAffinities(ctx, paths))
			require.Equal(t, paths, *f.setAffinitiesArg.Load())
		})
	})

	t.Run("Disconnect", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			require.Error(t, NewStation(f).Disconnect(ctx))
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			require.NoError(t, NewStation(f).Disconnect(ctx))
			require.True(t, f.disconnectCalled.Load())
		})
	})

	t.Run("ConnectHiddenNetwork", func(t *testing.T) {

		t.Run("EmptyNameIsInvalidArgument", func(t *testing.T) {
			for _, bad := range []string{"", "   "} {
				f := &fakeIwdbusStation{}
				err := NewStation(f).ConnectHiddenNetwork(ctx, bad)
				require.Error(t, err)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, KindInvalidArgument, ce.Kind)
				require.Equal(t, ResourceStation, ce.Resource)
				require.Nil(t, f.connectHiddenName.Load(), "backend must not be called for empty name")
			}
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrNoAgent)
			err := NewStation(f).ConnectHiddenNetwork(ctx, "HiddenSecured")
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrNoAgent)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			require.NoError(t, NewStation(f).ConnectHiddenNetwork(ctx, "HiddenNet"))
			require.Equal(t, "HiddenNet", *f.connectHiddenName.Load())
		})
	})

	t.Run("HiddenAccessPoints", func(t *testing.T) {

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).HiddenAccessPoints(ctx)
			require.Error(t, err)
		})

		t.Run("UnknownTypeIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			aps := []iwdbus.HiddenAccessPoint{{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -6000, Type: iwdbus.NetworkType("bogus")}}
			f.hiddenAPs.Store(&aps)
			_, err := NewStation(f).HiddenAccessPoints(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			aps := []iwdbus.HiddenAccessPoint{
				{Address: "  aa:bb:cc:dd:ee:ff  ", SignalStrength: -6000, Type: iwdbus.NetworkTypePSK},
				{Address: "11:22:33:44:55:66", SignalStrength: -7800, Type: iwdbus.NetworkTypeOpen},
			}
			f.hiddenAPs.Store(&aps)
			got, err := NewStation(f).HiddenAccessPoints(ctx)
			require.NoError(t, err)
			require.Equal(t, []HiddenAccessPoint{
				{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -6000, Type: NetworkTypePSK},
				{Address: "11:22:33:44:55:66", SignalStrength: -7800, Type: NetworkTypeOpen},
			}, got)
		})
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {

		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewStation(&fakeIwdbusStation{}).SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
		})

		t.Run("NormalizesEvent", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.subPropsEvent.Store(iwdbus.StationPropertiesChanged{
				Changed:     map[string]dbus.Variant{"State": dbus.MakeVariant("connecting")},
				Invalidated: []string{"ConnectedNetwork"},
			})

			var got StationPropertiesChanged
			_, err := NewStation(f).SubscribePropertiesChanged(ctx, func(ev StationPropertiesChanged) {
				got = ev
			})
			require.NoError(t, err)
			require.Equal(t, "connecting", got.Changed["State"])
			require.Equal(t, []string{"ConnectedNetwork"}, got.Invalidated)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
			require.ErrorIs(t, err, ErrCore)
		})
	})

	t.Run("SubscribeStateChanged", func(t *testing.T) {

		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewStation(&fakeIwdbusStation{}).SubscribeStateChanged(ctx, nil)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).SubscribeStateChanged(ctx, func(StationState) {})
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
			require.ErrorIs(t, err, ErrCore)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.subPropsEvent.Store(iwdbus.StationPropertiesChanged{
				Changed: map[string]dbus.Variant{"State": dbus.MakeVariant("roaming")},
			})

			var got StationState
			_, err := NewStation(f).SubscribeStateChanged(ctx, func(s StationState) {
				got = s
			})
			require.NoError(t, err)
			require.Equal(t, StationStateRoaming, got)
		})
	})

	t.Run("SubscribeScanningChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewStation(&fakeIwdbusStation{}).SubscribeScanningChanged(ctx, nil)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).SubscribeScanningChanged(ctx, func(bool) {})
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
			require.ErrorIs(t, err, ErrCore)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.subPropsEvent.Store(iwdbus.StationPropertiesChanged{
				Changed: map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)},
			})

			var got, fired = false, false
			_, err := NewStation(f).SubscribeScanningChanged(ctx, func(b bool) {
				got = b
				fired = true
			})
			require.NoError(t, err)
			require.True(t, fired)
			require.True(t, got)
		})
	})

	// Every method wraps a backend failure into a matchable core Error carrying
	// ResourceStation with the cause chained through ErrCore, so a wrong-resource
	// or swallowed-cause bug in any single method is caught (not just State).
	t.Run("BackendErrorWraps", func(t *testing.T) {
		t.Parallel()
		backendErr := errors.New("dbus boom")
		for _, tc := range []struct {
			name string
			call func(*Station) error
		}{
			{"State", func(s *Station) error { _, err := s.State(ctx); return err }},
			{"Scanning", func(s *Station) error { _, err := s.Scanning(ctx); return err }},
			{"ConnectedNetwork", func(s *Station) error { _, err := s.ConnectedNetwork(ctx); return err }},
			{"ConnectedAccessPoint", func(s *Station) error { _, err := s.ConnectedAccessPoint(ctx); return err }},
			{"Affinities", func(s *Station) error { _, err := s.Affinities(ctx); return err }},
			{"Properties", func(s *Station) error { _, err := s.Properties(ctx); return err }},
			{"Scan", func(s *Station) error { return s.Scan(ctx) }},
			{"OrderedNetworks", func(s *Station) error { _, err := s.OrderedNetworks(ctx); return err }},
			{"SetAffinities", func(s *Station) error { return s.SetAffinities(ctx, []string{"/x"}) }},
			{"Disconnect", func(s *Station) error { return s.Disconnect(ctx) }},
			{"ConnectHiddenNetwork", func(s *Station) error { return s.ConnectHiddenNetwork(ctx, "Hidden") }},
			{"HiddenAccessPoints", func(s *Station) error { _, err := s.HiddenAccessPoints(ctx); return err }},
			{"SubscribePropertiesChanged", func(s *Station) error {
				_, err := s.SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
				return err
			}},
			{"SubscribeStateChanged", func(s *Station) error {
				_, err := s.SubscribeStateChanged(ctx, func(StationState) {})
				return err
			}},
			{"SubscribeScanningChanged", func(s *Station) error {
				_, err := s.SubscribeScanningChanged(ctx, func(bool) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				f := &fakeIwdbusStation{}
				f.state.Store(iwdbus.StationStateConnected)
				f.setErr(backendErr)
				err := tc.call(NewStation(f))
				require.Error(t, err)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, ResourceStation, ce.Resource)
				require.ErrorIs(t, err, backendErr)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})

	// Every method guards a nil (uninitialized) receiver, returning a matchable
	// ErrStationNotInitialized (which wraps ErrCore) rather than panicking.
	t.Run("Uninitialized", func(t *testing.T) {
		var s *Station
		for _, tc := range []struct {
			name string
			call func() error
		}{
			{"State", func() error { _, err := s.State(ctx); return err }},
			{"Scanning", func() error { _, err := s.Scanning(ctx); return err }},
			{"ConnectedNetwork", func() error { _, err := s.ConnectedNetwork(ctx); return err }},
			{"ConnectedAccessPoint", func() error { _, err := s.ConnectedAccessPoint(ctx); return err }},
			{"Affinities", func() error { _, err := s.Affinities(ctx); return err }},
			{"Properties", func() error { _, err := s.Properties(ctx); return err }},
			{"Scan", func() error { return s.Scan(ctx) }},
			{"OrderedNetworks", func() error { _, err := s.OrderedNetworks(ctx); return err }},
			{"SetAffinities", func() error { return s.SetAffinities(ctx, []string{"/x"}) }},
			{"Disconnect", func() error { return s.Disconnect(ctx) }},
			{"ConnectHiddenNetwork", func() error { return s.ConnectHiddenNetwork(ctx, "x") }},
			{"HiddenAccessPoints", func() error { _, err := s.HiddenAccessPoints(ctx); return err }},
			{"SubscribePropertiesChanged", func() error {
				_, err := s.SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
				return err
			}},
			{"SubscribeScanningChanged", func() error {
				_, err := s.SubscribeScanningChanged(ctx, func(bool) {})
				return err
			}},
			{"SubscribeStateChanged", func() error {
				_, err := s.SubscribeStateChanged(ctx, func(StationState) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				require.ErrorIs(t, err, ErrStationNotInitialized)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})

	t.Run("ParseStationState", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			state, err := ParseStationState("connected")
			require.NoError(t, err)
			require.Equal(t, StationStateConnected, state)
		})

		t.Run("Invalid", func(t *testing.T) {
			state, err := ParseStationState("bogus")
			require.Error(t, err)
			require.Equal(t, StationStateUnknown, state)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
			require.Equal(t, ResourceStation, ce.Resource)
			require.ErrorIs(t, err, ErrCore)
			require.Contains(t, err.Error(), "invalid station state")
		})
	})
}

func TestStation_Core_NewSubscribes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := "/net/connman/iwd/0/3/ssid_psk"

	t.Run("ConnectedNetworkChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeIwdbusStation{}
		f.connNetEvnt.Store(&optStringEvent{v: &path})

		got := make(chan *string, 1)
		_, err := NewStation(f).SubscribeConnectedNetworkChanged(ctx, func(p *string) { got <- p })
		require.NoError(t, err)
		require.Equal(t, path, *<-got)
	})

	t.Run("ConnectedNetworkChanged delivers nil on disconnect", func(t *testing.T) {
		t.Parallel()
		f := &fakeIwdbusStation{}
		f.connNetEvnt.Store(&optStringEvent{v: nil})

		got := make(chan *string, 1)
		_, err := NewStation(f).SubscribeConnectedNetworkChanged(ctx, func(p *string) { got <- p })
		require.NoError(t, err)
		require.Nil(t, <-got)
	})

	t.Run("ConnectedAccessPointChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeIwdbusStation{}
		f.connAPEvnt.Store(&optStringEvent{v: &path})

		got := make(chan *string, 1)
		_, err := NewStation(f).SubscribeConnectedAccessPointChanged(ctx, func(p *string) { got <- p })
		require.NoError(t, err)
		require.Equal(t, path, *<-got)
	})

	t.Run("AffinitiesChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeIwdbusStation{}
		f.affinityEvnt.Store(&stringSliceEvent{v: []string{path}})

		got := make(chan []string, 1)
		_, err := NewStation(f).SubscribeAffinitiesChanged(ctx, func(p []string) { got <- p })
		require.NoError(t, err)
		require.Equal(t, []string{path}, <-got)
	})

	t.Run("Guards", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name    string
			nilFn   func(*Station) error
			backend func(*Station) error
		}{
			{"ConnectedNetworkChanged",
				func(s *Station) error { _, err := s.SubscribeConnectedNetworkChanged(ctx, nil); return err },
				func(s *Station) error {
					_, err := s.SubscribeConnectedNetworkChanged(ctx, func(*string) {})
					return err
				}},
			{"ConnectedAccessPointChanged",
				func(s *Station) error { _, err := s.SubscribeConnectedAccessPointChanged(ctx, nil); return err },
				func(s *Station) error {
					_, err := s.SubscribeConnectedAccessPointChanged(ctx, func(*string) {})
					return err
				}},
			{"AffinitiesChanged",
				func(s *Station) error { _, err := s.SubscribeAffinitiesChanged(ctx, nil); return err },
				func(s *Station) error {
					_, err := s.SubscribeAffinitiesChanged(ctx, func([]string) {})
					return err
				}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				// nil callback
				err := tc.nilFn(NewStation(&fakeIwdbusStation{}))
				require.Error(t, err)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, KindInvalidArgument, ce.Kind)

				// backend error
				f := &fakeIwdbusStation{}
				f.setErr(errors.New("subscribe failed"))
				require.Error(t, tc.backend(NewStation(f)))

				// nil receiver
				var nilStation *Station
				require.Error(t, tc.backend(nilStation))
			})
		}
	})
}
