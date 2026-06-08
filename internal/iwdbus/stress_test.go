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

	var hits int64
	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = src.intro.RegisterSignalHandler(
				"net.connman.iwd.Adapter",
				"ModeChanged",
				func(*dbus.Signal) {
					atomic.AddInt64(&hits, 1)
				},
			)
		}(i)
	}

	for range 500 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src.emit("net.connman.iwd.Adapter", "ModeChanged", "ap")
		}()
	}

	wg.Wait()

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&hits) >= 10
	}, time.Second, 10*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_MultiHandlerFanout(t *testing.T) {
	src := newFakeSignalSource(t)

	const handlers = 50
	const signals = 500

	var exactCount int64
	var wildcardCount int64

	for range handlers {
		require.NoError(t,
			src.intro.RegisterSignalHandler(
				"net.connman.iwd.Adapter",
				"PoweredChanged",
				func(*dbus.Signal) {
					atomic.AddInt64(&exactCount, 1)
				},
			),
		)
	}

	require.NoError(t,
		src.intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
			atomic.AddInt64(&wildcardCount, 1)
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
		return atomic.LoadInt64(&exactCount) == int64(handlers*signals) &&
			atomic.LoadInt64(&wildcardCount) == int64(signals)
	}, 2*time.Second, 10*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_MultiInterfaceSameMember(t *testing.T) {
	src := newFakeSignalSource(t)

	var adapterHits, stationHits int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"StateChanged",
			func(*dbus.Signal) {
				atomic.AddInt64(&adapterHits, 1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Station",
			"StateChanged",
			func(*dbus.Signal) {
				atomic.AddInt64(&stationHits, 1)
			},
		),
	)

	for range 300 {
		src.emit("net.connman.iwd.Adapter", "StateChanged", "on")
		src.emit("net.connman.iwd.Station", "StateChanged", "off")
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&adapterHits) == 300 && atomic.LoadInt64(&stationHits) == 300
	}, time.Second, 5*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_SlowHandlerDoesNotBlock(t *testing.T) {
	src := newFakeSignalSource(t)

	var fastHits int64
	var slowHits int64

	// Fast handler should keep firing even if slow handler is sleeping.
	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				atomic.AddInt64(&fastHits, 1)
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
				atomic.AddInt64(&slowHits, 1)
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
		return atomic.LoadInt64(&fastHits) == signals
	}, 500*time.Millisecond, 10*time.Millisecond)

	// Slow handler will lag but must eventually catch up
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&slowHits) == signals
	}, 3*time.Second, 20*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_SlowWildcardDoesNotBlockExact(t *testing.T) {
	src := newFakeSignalSource(t)

	var exactHits int64
	var wildcardHits int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				atomic.AddInt64(&exactHits, 1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt64(&wildcardHits, 1)
		}),
	)

	const signals = 200
	for range signals {
		src.emit("net.connman.iwd.Adapter", "PoweredChanged", true)
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&exactHits) == signals
	}, 500*time.Millisecond, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&wildcardHits) == signals
	}, 3*time.Second, 20*time.Millisecond)
}

func TestStress_Iwdbus_Dispatch_SlowHandler_Close(t *testing.T) {
	src := newFakeSignalSource(t)

	var hits int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"PoweredChanged",
			func(*dbus.Signal) {
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt64(&hits, 1)
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
		return atomic.LoadInt64(&hits) > 0
	}, time.Second, 20*time.Millisecond)

	<-done // Close must eventually return
}

func TestStress_Iwdbus_Dispatch_MixedHandlers(t *testing.T) {
	src := newFakeSignalSource(t)

	var fastHits, slowHits, wildcardHits int64

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"StateChanged",
			func(*dbus.Signal) {
				atomic.AddInt64(&fastHits, 1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler(
			"net.connman.iwd.Adapter",
			"StateChanged",
			func(*dbus.Signal) {
				time.Sleep(3 * time.Millisecond)
				atomic.AddInt64(&slowHits, 1)
			},
		),
	)

	require.NoError(t,
		src.intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
			atomic.AddInt64(&wildcardHits, 1)
		}),
	)

	const signals = 300
	for range signals {
		src.emit("net.connman.iwd.Adapter", "StateChanged", "on")
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&fastHits) == signals
	}, time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&slowHits) == signals
	}, 2*time.Second, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&wildcardHits) == signals
	}, time.Second, 10*time.Millisecond)
}

func TestStress_Iwdbus_ParseIntrospectChildNames(t *testing.T) {
	// Hammer parseIntrospectChildNames with a variety of shapes concurrently.
	base := `<node><node name="phy0"/><node name="phy1"/><node name="phy2"/></node>`

	const N = 2000
	var wg sync.WaitGroup
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()

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
		}(i)
	}

	wg.Wait()
}
