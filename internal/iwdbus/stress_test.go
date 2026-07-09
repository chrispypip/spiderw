//go:build stress

package iwdbus

import (
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestStress_Iwdbus_ConcurrentRegisterAndEmit(t *testing.T) {
	src := newFakeSignalSource(t)

	var hits atomic.Int64
	var wg sync.WaitGroup

	for range 50 {
		wg.Go(func() {
			_ = src.intro.RegisterSignalHandler(
				"net.connman.iwd.Adapter",
				"ModeChanged",
				func(*dbus.Signal) {
					hits.Add(1)
				},
			)
		})
	}

	for range 500 {
		wg.Go(func() {
			src.emit("net.connman.iwd.Adapter", "ModeChanged", "ap")
		})
	}

	wg.Wait()

	require.Eventually(t, func() bool {
		return hits.Load() >= 10
	}, time.Second, 10*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_MultiHandlerFanout(t *testing.T) {
	src := newFakeSignalSource(t)

	const handlers = 50
	const signals = 500

	var exactCount atomic.Int64
	var wildcardCount atomic.Int64

	for range handlers {
		require.NoError(t,
			src.intro.RegisterSignalHandler(
				"net.connman.iwd.Adapter",
				"PoweredChanged",
				func(*dbus.Signal) {
					exactCount.Add(1)
				},
			),
		)
	}

	require.NoError(t,
		src.intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
			wildcardCount.Add(1)
		}),
	)

	for range signals {
		src.emit(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			true,
		)
	}

	require.Eventually(t, func() bool {
		return exactCount.Load() == int64(handlers*signals) &&
			wildcardCount.Load() == int64(signals)
	}, 2*time.Second, 10*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_MultiInterfaceSameMember(t *testing.T) {
	src := newFakeSignalSource(t)

	var adapterHits atomic.Int64
	var stationHits atomic.Int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"StateChanged",
			func(*dbus.Signal) {
				adapterHits.Add(1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Station",
			"StateChanged",
			func(*dbus.Signal) {
				stationHits.Add(1)
			},
		),
	)

	for range 300 {
		src.emit("net.connman.iwd.Adapter", "StateChanged", "on")
		src.emit("net.connman.iwd.Station", "StateChanged", "off")
	}

	require.Eventually(t, func() bool {
		return adapterHits.Load() == 300 && stationHits.Load() == 300
	}, time.Second, 5*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_SlowHandlerDoesNotBlock(t *testing.T) {
	src := newFakeSignalSource(t)

	var fastHits atomic.Int64
	var slowHits atomic.Int64

	// Fast handler should keep firing even if slow handler is sleeping.
	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				fastHits.Add(1)
			},
		),
	)

	// Slow handler simulates real-world expensive work.
	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				time.Sleep(5 * time.Millisecond)
				slowHits.Add(1)
			},
		),
	)

	// Emit a burst of signals
	const signals = 200
	for range signals {
		src.emit(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			true,
		)
	}

	// Fast handler should complete quickly
	require.Eventually(t, func() bool {
		return fastHits.Load() == signals
	}, 500*time.Millisecond, 10*time.Millisecond)

	// Slow handler will lag but must eventually catch up
	require.Eventually(t, func() bool {
		return slowHits.Load() == signals
	}, 3*time.Second, 20*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_SlowWildcardDoesNotBlockExact(t *testing.T) {
	src := newFakeSignalSource(t)

	var exactHits atomic.Int64
	var wildcardHits atomic.Int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				exactHits.Add(1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
			time.Sleep(5 * time.Millisecond)
			wildcardHits.Add(1)
		}),
	)

	const signals = 200
	for range signals {
		src.emit("net.connman.iwd.Adapter", "PoweredChanged", true)
	}

	require.Eventually(t, func() bool {
		return exactHits.Load() == signals
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return wildcardHits.Load() == signals
	}, 3*time.Second, 20*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_SlowHandler_Close(t *testing.T) {
	src := newFakeSignalSource(t)

	var hits atomic.Int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				time.Sleep(10 * time.Millisecond)
				hits.Add(1)
			},
		),
	)

	const signals = 50
	for range signals {
		src.emit("net.connman.iwd.Adapter", "PoweredChanged", true)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = src.intro.Close()
	}()

	require.Eventually(t, func() bool {
		return hits.Load() > 0
	}, time.Second, 20*time.Millisecond)

	<-done // Close must eventually return
}

func TestStress_Iwdbus_Dispatch_MixedHandlers(t *testing.T) {
	src := newFakeSignalSource(t)

	var fastHits atomic.Int64
	var slowHits atomic.Int64
	var wildcardHits atomic.Int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"StateChanged",
			func(*dbus.Signal) {
				fastHits.Add(1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"StateChanged",
			func(*dbus.Signal) {
				time.Sleep(3 * time.Millisecond)
				slowHits.Add(1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
			wildcardHits.Add(1)
		}),
	)

	const signals = 300
	for range signals {
		src.emit("net.connman.iwd.Adapter", "StateChanged", "on")
	}

	require.Eventually(t, func() bool {
		return fastHits.Load() == signals
	}, time.Second, 10*time.Millisecond)
}

func TestStress_Iwdbus_ParseIntrospectionChildNames(t *testing.T) {
	// Hammer parseIntrospectChildNames with a variety of shapes concurrently.
	base := `<node><node name="phy0"/><node name="phy1"/><node name="phy2"/></node>`

	const N = 2000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			xml := base
			if i%7 == 0 {
				xml = strings.ReplaceAll(base, "py", "  phy")
			}
			if i%13 == 0 {
				xml = `<node></node>`
			}
			if i%29 == 0 {
				xml = `<node><node name="nested"><node name="child"/></node></node>`
			}
			// Occasionally create malformed XML; errors are acceptable, panics are not.
			if rand.Intn(50) == 0 {
				xml = `<node><node name="unterminated">`
			}

			_, _ = parseIntrospectionChildNames(xml)
		})
	}

	wg.Wait()
}
