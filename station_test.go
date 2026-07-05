//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

func TestStation_Public(t *testing.T) {
	ctx := context.Background()

	t.Run("State", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.state.Store(core.StationStateConnected)
			s := newStation(f, "/net/connman/iwd/phy0/wlan0")

			state, err := s.State(ctx)
			require.NoError(t, err)
			require.Equal(t, StationStateConnected, state)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			_, err := newStation(f, "/p").State(ctx)
			require.Error(t, err)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Station)(nil).State(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})
	})

	t.Run("Scanning", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.scanning.Store(true)
		v, err := newStation(f, "/p").Scanning(ctx)
		require.NoError(t, err)
		require.True(t, v)
	})

	t.Run("ConnectedNetwork", func(t *testing.T) {
		t.Run("Connected", func(t *testing.T) {
			f := &fakeCoreStation{}
			path := "/net/connman/iwd/phy0/wlan0/net0"
			f.connectedNetwork.Store(&path)

			got, err := newStation(f, "/p").ConnectedNetwork(ctx)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, path, *got)
		})

		t.Run("Disconnected", func(t *testing.T) {
			f := &fakeCoreStation{}
			got, err := newStation(f, "/p").ConnectedNetwork(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})
	})

	t.Run("ConnectedAccessPoint", func(t *testing.T) {
		t.Run("Connected", func(t *testing.T) {
			f := &fakeCoreStation{}
			ap := "/net/connman/iwd/phy0/wlan0/abc123"
			f.connectedAccessPoint.Store(&ap)

			got, err := newStation(f, "/p").ConnectedAccessPoint(ctx)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, ap, *got)
		})

		t.Run("Absent", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p").ConnectedAccessPoint(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})
	})

	t.Run("Affinities", func(t *testing.T) {
		t.Run("Present", func(t *testing.T) {
			f := &fakeCoreStation{}
			affinities := []string{"/net/connman/iwd/phy0/wlan0/aaa"}
			f.affinities.Store(&affinities)

			got, err := newStation(f, "/p").Affinities(ctx)
			require.NoError(t, err)
			require.Equal(t, affinities, got)
		})

		t.Run("Absent", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p").Affinities(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})
	})

	t.Run("Properties", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.state.Store(core.StationStateConnected)
		f.scanning.Store(false)
		path := "/net/connman/iwd/phy0/wlan0/net0"
		f.connectedNetwork.Store(&path)
		ap := "/net/connman/iwd/phy0/wlan0/abc123"
		f.connectedAccessPoint.Store(&ap)
		affinities := []string{ap}
		f.affinities.Store(&affinities)

		props, err := newStation(f, "/p").Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, StationStateConnected, props.State)
		require.False(t, props.Scanning)
		require.NotNil(t, props.ConnectedNetwork)
		require.Equal(t, path, *props.ConnectedNetwork)
		require.NotNil(t, props.ConnectedAccessPoint)
		require.Equal(t, ap, *props.ConnectedAccessPoint)
		require.Equal(t, affinities, props.Affinities)
	})

	t.Run("Scan", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			require.NoError(t, newStation(f, "/p").Scan(ctx))
			require.True(t, f.scanCalled.Load())
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			require.Error(t, newStation(f, "/p").Scan(ctx))
		})

		t.Run("NilReceiver", func(t *testing.T) {
			require.ErrorIs(t, (*Station)(nil).Scan(ctx), ErrInternal)
		})
	})

	t.Run("OrderedNetworks", func(t *testing.T) {
		t.Run("ConvertsSignalToDBm", func(t *testing.T) {
			f := &fakeCoreStation{}
			nets := []core.OrderedNetwork{
				{Network: "/net/connman/iwd/phy0/wlan0/net0", SignalStrength: -6000},
				{Network: "/net/connman/iwd/phy0/wlan0/net1", SignalStrength: -7250},
			}
			f.orderedNetworks.Store(&nets)

			got, err := newStation(f, "/p").OrderedNetworks(ctx)
			require.NoError(t, err)
			require.Equal(t, []OrderedNetwork{
				{Network: "/net/connman/iwd/phy0/wlan0/net0", SignalStrength: -60},
				{Network: "/net/connman/iwd/phy0/wlan0/net1", SignalStrength: -72.5},
			}, got)
		})

		t.Run("Empty", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p").OrderedNetworks(ctx)
			require.NoError(t, err)
			require.Empty(t, got)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			_, err := newStation(f, "/p").OrderedNetworks(ctx)
			require.Error(t, err)
		})
	})

	t.Run("SetAffinities", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			paths := []string{"/net/connman/iwd/phy0/wlan0/aaa"}
			require.NoError(t, newStation(f, "/p").SetAffinities(ctx, paths))
			require.Equal(t, paths, *f.setAffinitiesArg.Load())
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			require.Error(t, newStation(f, "/p").SetAffinities(ctx, []string{"/x"}))
		})

		t.Run("NotSupportedMatchable", func(t *testing.T) {
			// A driver rejection surfaced by iwd stays matchable as
			// ErrNotSupported through the public boundary.
			f := &fakeCoreStation{}
			f.setErr(core.ErrNotSupported)
			err := newStation(f, "/p").SetAffinities(ctx, []string{"/net/connman/iwd/0/3/net/cc28aad1fed0"})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrNotSupported)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			require.ErrorIs(t, (*Station)(nil).SetAffinities(ctx, []string{"/x"}), ErrInternal)
		})
	})

	t.Run("SubscribeStateChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := newStation(&fakeCoreStation{}, "/p").SubscribeStateChanged(ctx, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidArgument)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.subPropsEvent.Store(core.StationPropertiesChanged{
				Changed: map[string]any{"State": "roaming"},
			})

			var got StationState
			_, err := newStation(f, "/p").SubscribeStateChanged(ctx, func(s StationState) {
				got = s
			})
			require.NoError(t, err)
			require.Equal(t, StationStateRoaming, got)
		})
	})

	t.Run("SubscribeScanningChanged", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.subPropsEvent.Store(core.StationPropertiesChanged{
			Changed: map[string]any{"Scanning": true},
		})

		var got, fired = false, false
		_, err := newStation(f, "/p").SubscribeScanningChanged(ctx, func(b bool) {
			got = b
			fired = true
		})
		require.NoError(t, err)
		require.True(t, fired)
		require.True(t, got)
	})

	t.Run("Path", func(t *testing.T) {
		require.Equal(t, "", (*Station)(nil).Path())
		require.Equal(t, "/p", newStation(&fakeCoreStation{}, "/p").Path())
	})
}

func TestClientStation(t *testing.T) {
	ctx := context.Background()

	newStationClient := func(factory func(ctx context.Context, path string) (core.StationIface, error)) *Client {
		if factory == nil {
			factory = func(_ context.Context, path string) (core.StationIface, error) {
				fs := &fakeCoreStation{}
				fs.state.Store(core.StationStateConnected)
				return fs, nil
			}
		}
		wire := &connect.Wiring{
			Conn:           &dbus.Conn{},
			Daemon:         &fakeCoreDaemon{},
			Cleanup:        func() error { return nil },
			StationFactory: factory,
		}
		return &Client{daemon: newDaemon(&fakeCoreDaemon{}), wire: wire, cleanup: wire.Cleanup}
	}

	t.Run("Success", func(t *testing.T) {
		c := newStationClient(nil)
		s, err := c.Station(ctx, "/net/connman/iwd/phy0/wlan0")
		require.NoError(t, err)
		require.NotNil(t, s)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", s.Path())
	})

	t.Run("WiringErrorMapsToPublicError", func(t *testing.T) {
		base := errors.New("station unavailable")
		c := newStationClient(func(_ context.Context, _ string) (core.StationIface, error) {
			return nil, base
		})
		s, err := c.Station(ctx, "/net/connman/iwd/phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.ErrorIs(t, err, base)
	})

	t.Run("Closed", func(t *testing.T) {
		c := newStationClient(nil)
		require.NoError(t, c.Close())

		s, err := c.Station(ctx, "/net/connman/iwd/phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidState)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		s, err := c.Station(ctx, "/p")
		require.Nil(t, s)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestClientAllStations(t *testing.T) {
	ctx := context.Background()

	newAllStationsClient := func(
		refs []core.StationRef,
		daemonErr error,
		factory func(ctx context.Context, path string) (core.StationIface, error),
	) *Client {
		fakeDaemon := &fakeCoreDaemon{}
		fakeDaemon.setStations(refs)
		if daemonErr != nil {
			fakeDaemon.setErr(daemonErr)
		}
		if factory == nil {
			factory = func(_ context.Context, path string) (core.StationIface, error) {
				fs := &fakeCoreStation{}
				fs.state.Store(core.StationStateConnected)
				return fs, nil
			}
		}
		wire := &connect.Wiring{
			Conn:           &dbus.Conn{},
			Daemon:         fakeDaemon,
			Cleanup:        func() error { return nil },
			StationFactory: factory,
		}
		return &Client{daemon: newDaemon(fakeDaemon), wire: wire, cleanup: wire.Cleanup}
	}

	t.Run("Success", func(t *testing.T) {
		refs := []core.StationRef{
			{Path: "/net/connman/iwd/phy0/wlan0"},
			{Path: "/net/connman/iwd/phy1/wlan1"},
		}
		c := newAllStationsClient(refs, nil, nil)

		stations, err := c.AllStations(ctx)
		require.NoError(t, err)
		require.Len(t, stations, len(refs))
		for i, s := range stations {
			require.NotNil(t, s)
			require.Equal(t, refs[i].Path, s.Path())
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := newAllStationsClient(nil, nil, nil)
		stations, err := c.AllStations(ctx)
		require.NoError(t, err)
		require.NotNil(t, stations)
		require.Empty(t, stations)
	})

	t.Run("DaemonError", func(t *testing.T) {
		c := newAllStationsClient(nil, errors.New("enum failed"), nil)
		_, err := c.AllStations(ctx)
		require.Error(t, err)
	})
}
