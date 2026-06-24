//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync/atomic"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/core/testutil"
)

type fakeCoreDaemonError struct {
	err error
}

type fakeCoreDaemon struct {
	testutil.UnimplementedCoreDaemon

	info     atomic.Pointer[core.DaemonInfo]
	version  atomic.Pointer[string]
	stateDir atomic.Pointer[string]
	ncfg     atomic.Bool
	adapters atomic.Pointer[[]core.AdapterRef]
	devices  atomic.Pointer[[]core.DeviceRef]
	bsses    atomic.Pointer[[]core.BasicServiceSetRef]
	networks atomic.Pointer[[]core.NetworkRef]
	err      atomic.Pointer[fakeCoreDaemonError]
}

func (f *fakeCoreDaemon) setInfo(info *core.DaemonInfo) {
	if info == nil {
		f.info.Store(nil)
		return
	}

	infoCopy := *info
	f.info.Store(&infoCopy)
}

func (f *fakeCoreDaemon) setInfoVersion(version string) {
	versionCopy := version
	f.version.Store(&versionCopy)
}

func (f *fakeCoreDaemon) setInfoStateDirectory(stateDir string) {
	stateDirCopy := stateDir
	f.stateDir.Store(&stateDirCopy)
}

func (f *fakeCoreDaemon) setInfoNetworkConfigurationEnaled(enabled bool) {
	f.ncfg.Store(enabled)
}

func (f *fakeCoreDaemon) setAdapters(adapters []core.AdapterRef) {
	if adapters == nil {
		f.adapters.Store(nil)
		return
	}

	adaptersCopy := append([]core.AdapterRef(nil), adapters...)
	f.adapters.Store(&adaptersCopy)
}

func (f *fakeCoreDaemon) setDevices(devices []core.DeviceRef) {
	if devices == nil {
		f.devices.Store(nil)
		return
	}

	devicesCopy := append([]core.DeviceRef(nil), devices...)
	f.devices.Store(&devicesCopy)
}

func (f *fakeCoreDaemon) setBasicServiceSets(bsses []core.BasicServiceSetRef) {
	if bsses == nil {
		f.bsses.Store(nil)
		return
	}

	bssesCopy := append([]core.BasicServiceSetRef(nil), bsses...)
	f.bsses.Store(&bssesCopy)
}

func (f *fakeCoreDaemon) setNetworks(networks []core.NetworkRef) {
	if networks == nil {
		f.networks.Store(nil)
		return
	}

	networksCopy := append([]core.NetworkRef(nil), networks...)
	f.networks.Store(&networksCopy)
}

func (f *fakeCoreDaemon) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}

	f.err.Store(&fakeCoreDaemonError{err: err})
}

func (f *fakeCoreDaemon) loadErr() error {
	if box := f.err.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeCoreDaemon) Info(ctx context.Context) (*core.DaemonInfo, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	info := f.info.Load()
	if info == nil {
		return nil, nil
	}

	infoCopy := *info
	return &infoCopy, nil
}

func (f *fakeCoreDaemon) Version(ctx context.Context) (string, error) {
	if err := f.loadErr(); err != nil {
		return "", err
	}

	version := f.version.Load()
	if version == nil {
		return "", nil
	}
	return *version, nil
}

func (f *fakeCoreDaemon) StateDirectory(ctx context.Context) (string, error) {
	if err := f.loadErr(); err != nil {
		return "", err
	}

	stateDir := f.stateDir.Load()
	if stateDir == nil {
		return "", nil
	}
	return *stateDir, nil
}

func (f *fakeCoreDaemon) NetworkConfigurationEnabled(ctx context.Context) (bool, error) {
	if err := f.loadErr(); err != nil {
		return false, err
	}

	return f.ncfg.Load(), nil
}

func (f *fakeCoreDaemon) Adapters(ctx context.Context) ([]core.AdapterRef, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	adapters := f.adapters.Load()
	if adapters == nil {
		return nil, nil
	}
	return append([]core.AdapterRef(nil), (*adapters)...), nil
}

func (f *fakeCoreDaemon) Devices(ctx context.Context) ([]core.DeviceRef, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	devices := f.devices.Load()
	if devices == nil {
		return nil, nil
	}
	return append([]core.DeviceRef(nil), (*devices)...), nil
}

func (f *fakeCoreDaemon) BasicServiceSets(ctx context.Context) ([]core.BasicServiceSetRef, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	bsses := f.bsses.Load()
	if bsses == nil {
		return nil, nil
	}
	return append([]core.BasicServiceSetRef(nil), (*bsses)...), nil
}

func (f *fakeCoreDaemon) Networks(ctx context.Context) ([]core.NetworkRef, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	networks := f.networks.Load()
	if networks == nil {
		return nil, nil
	}
	return append([]core.NetworkRef(nil), (*networks)...), nil
}
