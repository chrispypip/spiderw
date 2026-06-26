//go:build unit || race || stress

package core

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/internal/iwdbus/testutil"
)

type fakeIwdbusDaemonError struct {
	err error
}

// fakeIwdbusDaemon implements only GetInfo() for testing.
// Its state is atomic so race/stress tests can safely call it from multiple
// goroutines while tests update the bake between operations.
type fakeIwdbusDaemon struct {
	testutil.UnimplementedIwdbusDaemon

	info          atomic.Pointer[iwdbus.DaemonInfo]
	adapters      atomic.Pointer[[]iwdbus.AdapterRef]
	devices       atomic.Pointer[[]iwdbus.DeviceRef]
	bsses         atomic.Pointer[[]iwdbus.BasicServiceSetRef]
	networks      atomic.Pointer[[]iwdbus.NetworkRef]
	knownNetworks atomic.Pointer[[]iwdbus.KnownNetworkRef]
	err           atomic.Pointer[fakeIwdbusDaemonError]
}

func fakeIwdbusDaemonWithInfo(info *iwdbus.DaemonInfo) *fakeIwdbusDaemon {
	f := &fakeIwdbusDaemon{}
	f.setInfo(info)
	return f
}

func (f *fakeIwdbusDaemon) setInfo(info *iwdbus.DaemonInfo) *fakeIwdbusDaemon {
	f.info.Store(cloneIwdbusDaemonInfo(info))
	return f
}

func (f *fakeIwdbusDaemon) setAdapters(adapters []iwdbus.AdapterRef) *fakeIwdbusDaemon {
	cloned := cloneIwdbusAdapterRefs(adapters)
	f.adapters.Store(&cloned)
	return f
}

func (f *fakeIwdbusDaemon) setDevices(devices []iwdbus.DeviceRef) *fakeIwdbusDaemon {
	cloned := cloneIwdbusDeviceRefs(devices)
	f.devices.Store(&cloned)
	return f
}

func (f *fakeIwdbusDaemon) setBasicServiceSets(bsses []iwdbus.BasicServiceSetRef) *fakeIwdbusDaemon {
	cloned := cloneIwdbusBSSRefs(bsses)
	f.bsses.Store(&cloned)
	return f
}

func (f *fakeIwdbusDaemon) setNetworks(networks []iwdbus.NetworkRef) *fakeIwdbusDaemon {
	cloned := cloneIwdbusNetworkRefs(networks)
	f.networks.Store(&cloned)
	return f
}

func (f *fakeIwdbusDaemon) setKnownNetworks(known []iwdbus.KnownNetworkRef) *fakeIwdbusDaemon {
	cloned := cloneIwdbusKnownNetworkRefs(known)
	f.knownNetworks.Store(&cloned)
	return f
}

func (f *fakeIwdbusDaemon) setErr(err error) *fakeIwdbusDaemon {
	if err == nil {
		f.err.Store(nil)
		return f
	}
	f.err.Store(&fakeIwdbusDaemonError{err: err})
	return f
}

func (f *fakeIwdbusDaemon) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeIwdbusDaemon) GetInfo(ctx context.Context) (*iwdbus.DaemonInfo, error) {
	return cloneIwdbusDaemonInfo(f.info.Load()), f.loadErr()
}

func (f *fakeIwdbusDaemon) GetAdapters(ctx context.Context) ([]iwdbus.AdapterRef, error) {
	ptr := f.adapters.Load()
	if ptr == nil {
		return nil, f.loadErr()
	}
	return cloneIwdbusAdapterRefs(*ptr), f.loadErr()
}

func (f *fakeIwdbusDaemon) GetDevices(ctx context.Context) ([]iwdbus.DeviceRef, error) {
	ptr := f.devices.Load()
	if ptr == nil {
		return nil, f.loadErr()
	}
	return cloneIwdbusDeviceRefs(*ptr), f.loadErr()
}

func (f *fakeIwdbusDaemon) GetBasicServiceSets(ctx context.Context) ([]iwdbus.BasicServiceSetRef, error) {
	ptr := f.bsses.Load()
	if ptr == nil {
		return nil, f.loadErr()
	}
	return cloneIwdbusBSSRefs(*ptr), f.loadErr()
}

func (f *fakeIwdbusDaemon) GetNetworks(ctx context.Context) ([]iwdbus.NetworkRef, error) {
	ptr := f.networks.Load()
	if ptr == nil {
		return nil, f.loadErr()
	}
	return cloneIwdbusNetworkRefs(*ptr), f.loadErr()
}

func (f *fakeIwdbusDaemon) GetKnownNetworks(ctx context.Context) ([]iwdbus.KnownNetworkRef, error) {
	ptr := f.knownNetworks.Load()
	if ptr == nil {
		return nil, f.loadErr()
	}
	return cloneIwdbusKnownNetworkRefs(*ptr), f.loadErr()
}

// newTestDaemon mirrors helpers used by internal/iwdbus tests.
// It returns a fully-initialized *Daemon backed by a concurrency-safe fake
// that always returns a valid DaemonInfo payload.
func newTestDaemon(t *testing.T) *Daemon {
	t.Helper()

	f := &fakeIwdbusDaemon{}
	f.setInfo(&iwdbus.DaemonInfo{
		Version:                     "1.0",
		StateDirectory:              "/var/lib/iwd",
		NetworkConfigurationEnabled: true,
	})

	d := NewDaemon(f)
	require.NotNil(t, d)

	return d
}

func cloneIwdbusDaemonInfo(info *iwdbus.DaemonInfo) *iwdbus.DaemonInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	return &cloned
}

func cloneIwdbusAdapterRefs(refs []iwdbus.AdapterRef) []iwdbus.AdapterRef {
	if refs == nil {
		return nil
	}
	out := make([]iwdbus.AdapterRef, len(refs))
	copy(out, refs)
	return out
}

func cloneIwdbusDeviceRefs(refs []iwdbus.DeviceRef) []iwdbus.DeviceRef {
	if refs == nil {
		return nil
	}
	out := make([]iwdbus.DeviceRef, len(refs))
	copy(out, refs)
	return out
}

func cloneIwdbusBSSRefs(refs []iwdbus.BasicServiceSetRef) []iwdbus.BasicServiceSetRef {
	if refs == nil {
		return nil
	}
	out := make([]iwdbus.BasicServiceSetRef, len(refs))
	copy(out, refs)
	return out
}

func cloneIwdbusNetworkRefs(refs []iwdbus.NetworkRef) []iwdbus.NetworkRef {
	if refs == nil {
		return nil
	}
	out := make([]iwdbus.NetworkRef, len(refs))
	copy(out, refs)
	return out
}

func cloneIwdbusKnownNetworkRefs(refs []iwdbus.KnownNetworkRef) []iwdbus.KnownNetworkRef {
	if refs == nil {
		return nil
	}
	out := make([]iwdbus.KnownNetworkRef, len(refs))
	copy(out, refs)
	return out
}
