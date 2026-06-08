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

	info     atomic.Pointer[iwdbus.DaemonInfo]
	adapters atomic.Pointer[[]iwdbus.AdapterRef]
	err      atomic.Pointer[fakeIwdbusDaemonError]
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
