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

		tree := fakeTree{netP: "PrivateNet", apP: "de:ad:be:ef:ca:fe"}
		s := newStation(f, "/p", "").withResolver(fakeResolver{tree: tree})
		props, err := s.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkRef{Path: netP, Name: "PrivateNet"}, *props.ConnectedNetwork)
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

	t.Run("PropertiesBackendError", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.setErr(core.WrapStationUnavailable("op", "boom", errors.New("x")))
		_, err := newStation(f, "/p", "").Properties(ctx)
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceStation, pe.Resource)
	})

	t.Run("PropertiesInvalidStateRejected", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.state.Store(core.StationState("garbage"))
		_, err := newStation(f, "/p", "").Properties(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
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

		t.Run("ResolverError", func(t *testing.T) {
			f := &fakeCoreStation{}
			nets := []core.OrderedNetwork{{Network: "/n", SignalStrength: -6000}}
			f.orderedNetworks.Store(&nets)
			s := newStation(f, "/p", "").withResolver(fakeResolver{err: errors.New("tree boom")})
			_, err := s.OrderedNetworks(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), "tree boom")
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
	})

	t.Run("Disconnect", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreStation{}
			require.NoError(t, newStation(f, "/p", "").Disconnect(ctx))
			require.True(t, f.disconnectCalled.Load())
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

		t.Run("InvalidTypeRejected", func(t *testing.T) {
			f := &fakeCoreStation{}
			aps := []core.HiddenAccessPoint{{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -6000, Type: core.NetworkType("garbage")}}
			f.hiddenAPs.Store(&aps)
			_, err := newStation(f, "/p", "").HiddenAccessPoints(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidArgument)
		})
	})

	t.Run("StateString", func(t *testing.T) {
		require.Equal(t, "connected", StationStateConnected.String())
		require.Equal(t, "disconnected", StationStateDisconnected.String())
	})

	t.Run("NewStation_NilCore", func(t *testing.T) {
		require.Nil(t, newStation(nil, "/p", "name"))
	})

	t.Run("StateInvalidRejectedAtBoundary", func(t *testing.T) {
		f := &fakeCoreStation{}
		f.state.Store(core.StationState("garbage"))
		_, err := newStation(f, "/p", "").State(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)

		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceStation, pe.Resource)
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := newStation(&fakeCoreStation{}, "/p", "").SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, KindInvalidArgument, pe.Kind)
			require.Equal(t, ResourceStation, pe.Resource)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Station)(nil).SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.subPropsEvent.Store(core.StationPropertiesChanged{
				Changed:     map[string]any{"State": "roaming"},
				Invalidated: []string{"ConnectedNetwork"},
			})

			var got StationPropertiesChanged
			_, err := newStation(f, "/p", "").SubscribePropertiesChanged(ctx, func(ev StationPropertiesChanged) { got = ev })
			require.NoError(t, err)
			require.Equal(t, "roaming", got.Changed["State"])
			require.Equal(t, []string{"ConnectedNetwork"}, got.Invalidated)
		})

		t.Run("BackendError", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(core.WrapStationUnavailable("op", "boom", errors.New("x")))
			_, err := newStation(f, "/p", "").SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceStation, pe.Resource)
		})
	})

	t.Run("SubscribeStateChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := newStation(&fakeCoreStation{}, "/p", "").SubscribeStateChanged(ctx, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidArgument)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Station)(nil).SubscribeStateChanged(ctx, func(StationState) {})
			require.ErrorIs(t, err, ErrInternal)
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

		t.Run("BackendError", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(core.WrapStationUnavailable("op", "boom", errors.New("x")))
			_, err := newStation(f, "/p", "").SubscribeStateChanged(ctx, func(StationState) {})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DropsUnrecognizedState", func(t *testing.T) {
			// The public wrapper defensively drops a state the lower layers should
			// never deliver rather than surfacing StationStateUnknown.
			f := &fakeCoreStation{}
			f.subPropsEvent.Store(core.StationPropertiesChanged{
				Changed: map[string]any{"State": "not-a-real-state"},
			})

			fired := false
			_, err := newStation(f, "/p", "").SubscribeStateChanged(ctx, func(StationState) { fired = true })
			require.NoError(t, err)
			require.False(t, fired)
		})
	})

	t.Run("SubscribeScanningChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := newStation(&fakeCoreStation{}, "/p", "").SubscribeScanningChanged(ctx, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInvalidArgument)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Station)(nil).SubscribeScanningChanged(ctx, func(bool) {})
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
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

		t.Run("BackendError", func(t *testing.T) {
			f := &fakeCoreStation{}
			f.setErr(core.WrapStationUnavailable("op", "boom", errors.New("x")))
			_, err := newStation(f, "/p", "").SubscribeScanningChanged(ctx, func(bool) {})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})
	})

	t.Run("Path", func(t *testing.T) {
		require.Empty(t, (*Station)(nil).Path())
		require.Equal(t, "/p", newStation(&fakeCoreStation{}, "/p", "").Path())
	})

	// A backend failure from any method must surface as a translated public
	// *Error carrying ResourceStation (not a leaked core error), so a method that
	// forgets to translate or reports the wrong resource is caught.
	t.Run("BackendErrorTranslates", func(t *testing.T) {
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
		} {
			t.Run(tc.name, func(t *testing.T) {
				f := &fakeCoreStation{}
				f.state.Store(core.StationStateConnected)
				f.setErr(core.WrapStationUnavailable("op", "boom", errors.New("x")))
				err := tc.call(newStation(f, "/p", ""))
				require.Error(t, err)
				var pe *Error
				require.ErrorAs(t, err, &pe)
				require.Equal(t, ResourceStation, pe.Resource)
			})
		}
	})

	// Every error-returning method guards a nil receiver, mapping to ErrInternal
	// rather than panicking.
	t.Run("NilReceiver", func(t *testing.T) {
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
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.ErrorIs(t, tc.call(), ErrInternal)
			})
		}
	})
}

func TestClient_Station(t *testing.T) {
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

func TestClient_AllStations(t *testing.T) {
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

func TestStation_NewSubscribes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := "/net/connman/iwd/0/3/ssid_psk"

	t.Run("ConnectedNetworkChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreStation{}
		f.connNetEvnt.Store(&optStringEvent{v: &path})

		got := make(chan *string, 1)
		_, err := newStation(f, "/s", "wlan0").SubscribeConnectedNetworkChanged(ctx, func(p *string) { got <- p })
		require.NoError(t, err)
		require.Equal(t, path, *<-got)
	})

	t.Run("ConnectedNetworkChanged delivers nil on disconnect", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreStation{}
		f.connNetEvnt.Store(&optStringEvent{v: nil})

		got := make(chan *string, 1)
		_, err := newStation(f, "/s", "wlan0").SubscribeConnectedNetworkChanged(ctx, func(p *string) { got <- p })
		require.NoError(t, err)
		require.Nil(t, <-got, "nil means disconnected")
	})

	t.Run("ConnectedAccessPointChanged observes a roam", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreStation{}
		f.connAPEvnt.Store(&optStringEvent{v: &path})

		got := make(chan *string, 1)
		_, err := newStation(f, "/s", "wlan0").SubscribeConnectedAccessPointChanged(ctx, func(p *string) { got <- p })
		require.NoError(t, err)
		require.Equal(t, path, *<-got)
	})

	t.Run("AffinitiesChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreStation{}
		f.affinityEvnt.Store(&stringSliceEvent{v: []string{path}})

		got := make(chan []string, 1)
		_, err := newStation(f, "/s", "wlan0").SubscribeAffinitiesChanged(ctx, func(p []string) { got <- p })
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

				require.Error(t, tc.nilFn(newStation(&fakeCoreStation{}, "/s", "wlan0")))

				f := &fakeCoreStation{}
				f.setErr(errors.New("subscribe failed"))
				require.Error(t, tc.backend(newStation(f, "/s", "wlan0")))

				var nilStation *Station
				require.Error(t, tc.backend(nilStation))
			})
		}
	})
}
