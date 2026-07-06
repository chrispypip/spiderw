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

// fakeTree/fakeResolver drive the friendly-ref resolution paths without a D-Bus
// connection. fakeTree is keyed by object path; every lookup shares the map.
type fakeTree map[string]string

func (t fakeTree) NetworkName(p string) string      { return t[p] }
func (t fakeTree) BSSAddress(p string) string       { return t[p] }
func (t fakeTree) DeviceName(p string) string       { return t[p] }
func (t fakeTree) AdapterName(p string) string      { return t[p] }
func (t fakeTree) KnownNetworkName(p string) string { return t[p] }

type fakeResolver struct {
	tree connect.Tree
	err  error
}

func (r fakeResolver) Resolve(context.Context) (connect.Tree, error) { return r.tree, r.err }

func TestStation_Public(t *testing.T) {
	ctx := context.Background()

	t.Run("State", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.state.Store(core.StationStateConnected)
			s := newStation(f, "/net/connman/iwd/phy0/wlan0", "")

			state, err := s.State(ctx)
			require.NoError(t, err)
			require.Equal(t, StationStateConnected, state)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			_, err := newStation(f, "/p", "").State(ctx)
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
		v, err := newStation(f, "/p", "").Scanning(ctx)
		require.NoError(t, err)
		require.True(t, v)
	})

	t.Run("Name", func(t *testing.T) {
		require.Equal(t, "wlan0",
			newStation(&fakeCoreStation{}, "/net/connman/iwd/phy0/wlan0", "wlan0").Name())
		require.Empty(t, newStation(&fakeCoreStation{}, "/p", "").Name())
		require.Empty(t, (*Station)(nil).Name())
	})

	t.Run("ConnectedNetwork", func(t *testing.T) {
		t.Run("Connected", func(t *testing.T) {
			f := &fakeCoreStation{}
			path := "/net/connman/iwd/phy0/wlan0/net0"
			f.connectedNetwork.Store(&path)

			got, err := newStation(f, "/p", "").ConnectedNetwork(ctx)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, path, *got)
		})

		t.Run("Disconnected", func(t *testing.T) {
			f := &fakeCoreStation{}
			got, err := newStation(f, "/p", "").ConnectedNetwork(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})
	})

	t.Run("ConnectedAccessPoint", func(t *testing.T) {
		t.Run("Connected", func(t *testing.T) {
			f := &fakeCoreStation{}
			ap := "/net/connman/iwd/phy0/wlan0/abc123"
			f.connectedAccessPoint.Store(&ap)

			got, err := newStation(f, "/p", "").ConnectedAccessPoint(ctx)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, ap, *got)
		})

		t.Run("Absent", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p", "").ConnectedAccessPoint(ctx)
			require.NoError(t, err)
			require.Nil(t, got)
		})
	})

	t.Run("Affinities", func(t *testing.T) {
		t.Run("Present", func(t *testing.T) {
			f := &fakeCoreStation{}
			affinities := []string{"/net/connman/iwd/phy0/wlan0/aaa"}
			f.affinities.Store(&affinities)

			got, err := newStation(f, "/p", "").Affinities(ctx)
			require.NoError(t, err)
			require.Equal(t, affinities, got)
		})

		t.Run("Absent", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p", "").Affinities(ctx)
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

		// No resolver on this station: refs carry Path, with Name/Address empty.
		props, err := newStation(f, "/p", "").Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, StationStateConnected, props.State)
		require.False(t, props.Scanning)
		require.NotNil(t, props.ConnectedNetwork)
		require.Equal(t, NetworkRef{Path: path}, *props.ConnectedNetwork)
		require.NotNil(t, props.ConnectedAccessPoint)
		require.Equal(t, BasicServiceSetRef{Path: ap}, *props.ConnectedAccessPoint)
		require.Equal(t, []BasicServiceSetRef{{Path: ap}}, props.Affinities)
	})

	t.Run("PropertiesResolvesNames", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.state.Store(core.StationStateConnected)
		netP := "/net/connman/iwd/0/3/ssid_psk"
		f.connectedNetwork.Store(&netP)
		apP := "/net/connman/iwd/0/3/ssid_psk/deadbeefcafe"
		f.connectedAccessPoint.Store(&apP)
		aff := []string{apP}
		f.affinities.Store(&aff)

		tree := fakeTree{netP: "ShadowGate", apP: "de:ad:be:ef:ca:fe"}
		s := newStation(f, "/p", "").withResolver(fakeResolver{tree: tree})
		props, err := s.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkRef{Path: netP, Name: "ShadowGate"}, *props.ConnectedNetwork)
		require.Equal(t, BasicServiceSetRef{Path: apP, Address: "de:ad:be:ef:ca:fe"}, *props.ConnectedAccessPoint)
		require.Equal(t, []BasicServiceSetRef{{Path: apP, Address: "de:ad:be:ef:ca:fe"}}, props.Affinities)
	})

	t.Run("PropertiesPropagatesResolverError", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.state.Store(core.StationStateConnected)
		s := newStation(f, "/p", "").withResolver(fakeResolver{err: errors.New("tree boom")})
		_, err := s.Properties(ctx)
		require.Error(t, err)
	})

	t.Run("Scan", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			require.NoError(t, newStation(f, "/p", "").Scan(ctx))
			require.True(t, f.scanCalled.Load())
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			require.Error(t, newStation(f, "/p", "").Scan(ctx))
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

			// No resolver on this station, so refs are path-only (Name "").
			got, err := newStation(f, "/p", "").OrderedNetworks(ctx)
			require.NoError(t, err)
			require.Equal(t, []OrderedNetwork{
				{NetworkRef: NetworkRef{Path: "/net/connman/iwd/phy0/wlan0/net0"}, SignalStrength: -60},
				{NetworkRef: NetworkRef{Path: "/net/connman/iwd/phy0/wlan0/net1"}, SignalStrength: -72.5},
			}, got)
		})

		t.Run("Empty", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p", "").OrderedNetworks(ctx)
			require.NoError(t, err)
			require.Empty(t, got)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			_, err := newStation(f, "/p", "").OrderedNetworks(ctx)
			require.Error(t, err)
		})
	})

	t.Run("SetAffinities", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			paths := []string{"/net/connman/iwd/phy0/wlan0/aaa"}
			require.NoError(t, newStation(f, "/p", "").SetAffinities(ctx, paths))
			require.Equal(t, paths, *f.setAffinitiesArg.Load())
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			require.Error(t, newStation(f, "/p", "").SetAffinities(ctx, []string{"/x"}))
		})

		t.Run("NotSupportedMatchable", func(t *testing.T) {
			// A driver rejection surfaced by iwd stays matchable as
			// ErrNotSupported through the public boundary.
			f := &fakeCoreStation{}
			f.setErr(core.ErrNotSupported)
			err := newStation(f, "/p", "").SetAffinities(ctx, []string{"/net/connman/iwd/0/3/net/a0b1c2d3e4f5"})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrNotSupported)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			require.ErrorIs(t, (*Station)(nil).SetAffinities(ctx, []string{"/x"}), ErrInternal)
		})
	})

	t.Run("Disconnect", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			require.NoError(t, newStation(f, "/p", "").Disconnect(ctx))
			require.True(t, f.disconnectCalled.Load())
		})

		t.Run("NilReceiver", func(t *testing.T) {
			require.ErrorIs(t, (*Station)(nil).Disconnect(ctx), ErrInternal)
		})
	})

	t.Run("ConnectHiddenNetwork", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			require.NoError(t, newStation(f, "/p", "").ConnectHiddenNetwork(ctx, "HiddenNet"))
			require.Equal(t, "HiddenNet", *f.connectHiddenName.Load())
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			require.Error(t, newStation(f, "/p", "").ConnectHiddenNetwork(ctx, "HiddenNet"))
		})

		t.Run("NilReceiver", func(t *testing.T) {
			require.ErrorIs(t, (*Station)(nil).ConnectHiddenNetwork(ctx, "x"), ErrInternal)
		})
	})

	t.Run("HiddenAccessPoints", func(t *testing.T) {
		t.Run("ConvertsSignalAndType", func(t *testing.T) {
			f := &fakeCoreStation{}
			aps := []core.HiddenAccessPoint{
				{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -6000, Type: core.NetworkTypePSK},
				{Address: "11:22:33:44:55:66", SignalStrength: -7250, Type: core.NetworkTypeOpen},
			}
			f.hiddenAPs.Store(&aps)

			got, err := newStation(f, "/p", "").HiddenAccessPoints(ctx)
			require.NoError(t, err)
			require.Equal(t, []HiddenAccessPoint{
				{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -60, Type: NetworkTypePSK},
				{Address: "11:22:33:44:55:66", SignalStrength: -72.5, Type: NetworkTypeOpen},
			}, got)
		})

		t.Run("Empty", func(t *testing.T) {
			got, err := newStation(&fakeCoreStation{}, "/p", "").HiddenAccessPoints(ctx)
			require.NoError(t, err)
			require.Empty(t, got)
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(errors.New("boom"))
			_, err := newStation(f, "/p", "").HiddenAccessPoints(ctx)
			require.Error(t, err)
		})
	})

	t.Run("SubscribeStateChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := newStation(&fakeCoreStation{}, "/p", "").SubscribeStateChanged(ctx, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidArgument)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.subPropsEvent.Store(core.StationPropertiesChanged{
				Changed: map[string]any{"State": "roaming"},
			})

			var got StationState
			_, err := newStation(f, "/p", "").SubscribeStateChanged(ctx, func(s StationState) {
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
		_, err := newStation(f, "/p", "").SubscribeScanningChanged(ctx, func(b bool) {
			got = b
			fired = true
		})
		require.NoError(t, err)
		require.True(t, fired)
		require.True(t, got)
	})

	t.Run("Path", func(t *testing.T) {
		require.Equal(t, "", (*Station)(nil).Path())
		require.Equal(t, "/p", newStation(&fakeCoreStation{}, "/p", "").Path())
	})
}

func TestClientStation(t *testing.T) {
	ctx := context.Background()

	newStationClient := func(factory func(ctx context.Context, path string) (core.StationIface, error)) *Client {
		if factory == nil {
			factory = func(ctx context.Context, path string) (core.StationIface, error) {
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
		c := newStationClient(func(ctx context.Context, path string) (core.StationIface, error) {
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
			factory = func(ctx context.Context, path string) (core.StationIface, error) {
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
