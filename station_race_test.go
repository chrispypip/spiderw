//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

func newRaceStation() *Station {
	f := &fakeCoreStation{}
	f.state.Store(core.StationStateConnected)
	f.scanning.Store(true)
	cn := "/net/connman/iwd/0/3/ssid_psk"
	ap := "/net/connman/iwd/0/3/ssid_psk/aabbccddeeff"
	f.connectedNetwork.Store(&cn)
	f.connectedAccessPoint.Store(&ap)
	aff := []string{ap}
	f.affinities.Store(&aff)
	return newStation(f, "/net/connman/iwd/0/3", "wlan0")
}

func TestRace_Public_Client_Station(t *testing.T) {
	wire := &connect.Wiring{
		Conn:    &dbus.Conn{},
		Daemon:  &fakeCoreDaemon{},
		Cleanup: func() error { return nil },
		StationFactory: func(ctx context.Context, path string) (core.StationIface, error) {
			fs := &fakeCoreStation{}
			fs.state.Store(core.StationStateConnected)
			return fs, nil
		},
	}
	client := &Client{daemon: newDaemon(&fakeCoreDaemon{}), wire: wire, cleanup: wire.Cleanup}

	const N = 100
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			s, err := client.Station(context.Background(), "/net/connman/iwd/0/3")
			require.NoError(t, err)
			require.NotNil(t, s)
		})
	}
	wg.Wait()
}

func TestRace_Public_Station_ConcurrentAccessors(t *testing.T) {
	station := newRaceStation()

	const N = 200
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 6 {
			case 0:
				_, _ = station.State(ctx)
			case 1:
				_, _ = station.Scanning(ctx)
			case 2:
				_, _ = station.ConnectedNetwork(ctx)
			case 3:
				_, _ = station.ConnectedAccessPoint(ctx)
			case 4:
				_, _ = station.Affinities(ctx)
			default:
				_, _ = station.Properties(ctx)
			}
		})
	}
	wg.Wait()
}

func TestRace_Public_Station_SetAffinitiesAndRead(t *testing.T) {
	station := newRaceStation()

	const N = 200
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 3 {
			case 0:
				_ = station.SetAffinities(ctx, []string{"/net/connman/iwd/0/3/ssid_psk/aabbccddeeff"})
			case 1:
				_, _ = station.Affinities(ctx)
			default:
				_, _ = station.Properties(ctx)
			}
		})
	}
	wg.Wait()
}

func TestRace_Public_Station_SubscribeStateChanged_Concurrent(t *testing.T) {
	station := newRaceStation()
	fs := station.core.(*fakeCoreStation)
	fs.subPropsEvent.Store(core.StationPropertiesChanged{Changed: map[string]any{"State": "roaming"}})

	const N = 200
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := station.SubscribeStateChanged(ctx, func(StationState) {})
			require.NoError(t, err)
		})
	}
	wg.Wait()
}

func TestRace_Public_Station_SubscribeScanningChanged_Concurrent(t *testing.T) {
	station := newRaceStation()
	fs := station.core.(*fakeCoreStation)
	fs.subPropsEvent.Store(core.StationPropertiesChanged{Changed: map[string]any{"Scanning": true}})

	const N = 200
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := station.SubscribeScanningChanged(ctx, func(bool) {})
			require.NoError(t, err)
		})
	}
	wg.Wait()
}

func TestRace_Public_Station_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	station := newRaceStation()
	fs := station.core.(*fakeCoreStation)
	fs.subPropsEvent.Store(core.StationPropertiesChanged{
		Changed:     map[string]any{"State": "roaming", "Scanning": true},
		Invalidated: []string{"ConnectedNetwork"},
	})

	const N = 200
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := station.SubscribePropertiesChanged(ctx, func(ev StationPropertiesChanged) {
				// User code may mutate its copy; the wrapper must not share maps
				// or slices across concurrent callbacks.
				if ev.Changed != nil {
					ev.Changed["user-mutation"] = 1
				}
				_ = ev.Invalidated
			})
			require.NoError(t, err)
		})
	}
	wg.Wait()
}

func TestRace_Public_Station_ContextCancel(t *testing.T) {
	station := newRaceStation()

	const N = 100
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			switch i % 5 {
			case 0:
				_, _ = station.State(ctx)
			case 1:
				_ = station.Scan(ctx)
			case 2:
				_, _ = station.Properties(ctx)
			case 3:
				_ = station.SetAffinities(ctx, []string{"/x"})
			default:
				_, _ = station.SubscribeStateChanged(ctx, func(StationState) {})
			}
		})
	}
	wg.Wait()
}
