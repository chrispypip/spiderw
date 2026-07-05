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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).State(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
			require.True(t, errors.Is(err, ErrCore))
		})

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
					require.True(t, errors.Is(err, tc.dbusErr))
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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).Scanning(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewStation(f).Scanning(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, iwdbus.ErrDBusMethod))
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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).ConnectedNetwork(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).ConnectedAccessPoint(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).Affinities(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).Properties(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			err := (*Station)(nil).Scan(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			f.setErr(iwdbus.ErrDBusMethod)
			err := NewStation(f).Scan(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, iwdbus.ErrDBusMethod))
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			require.NoError(t, NewStation(f).Scan(ctx))
			require.True(t, f.scanCalled.Load())
		})
	})

	t.Run("OrderedNetworks", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).OrderedNetworks(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			err := (*Station)(nil).SetAffinities(ctx, []string{"/x"})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			err := (*Station)(nil).Disconnect(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			err := (*Station)(nil).ConnectHiddenNetwork(ctx, "HiddenNet")
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
			require.True(t, errors.Is(err, iwdbus.ErrNoAgent))
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusStation{}
			require.NoError(t, NewStation(f).ConnectHiddenNetwork(ctx, "HiddenNet"))
			require.Equal(t, "HiddenNet", *f.connectHiddenName.Load())
		})
	})

	t.Run("HiddenAccessPoints", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).HiddenAccessPoints(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

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
	})

	t.Run("SubscribeStateChanged", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Station)(nil).SubscribeStateChanged(ctx, func(StationState) {})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStationNotInitialized))
		})

		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewStation(&fakeIwdbusStation{}).SubscribeStateChanged(ctx, nil)
			require.Error(t, err)
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
}
