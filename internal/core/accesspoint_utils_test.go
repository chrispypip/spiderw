//go:build unit || race || stress

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// fakeAccessPointRaw is a concurrency-safe accessPointRaw for core AccessPoint
// tests. Configured return fields are set at construction and only read;
// recorded call arguments are mutex-guarded so race and stress tests can hammer
// it.
type fakeAccessPointRaw struct {
	started         bool
	scanning        bool
	name            *string
	frequency       *uint32
	pairwiseCiphers []string
	groupCipher     *string
	ordered         []iwdbus.AccessPointOrderedNetwork
	props           *iwdbus.AccessPointProperties
	err             error
	subEvent        *iwdbus.AccessPointPropertiesChanged

	mu          sync.Mutex
	startedSSID string
	startedPSK  string
	profileSSID string
	stopCalled  bool
	scanCalled  bool
}

func (f *fakeAccessPointRaw) GetStarted(ctx context.Context) (bool, error)  { return f.started, f.err }
func (f *fakeAccessPointRaw) GetScanning(ctx context.Context) (bool, error) { return f.scanning, f.err }
func (f *fakeAccessPointRaw) GetName(ctx context.Context) (*string, error)  { return f.name, f.err }

func (f *fakeAccessPointRaw) GetFrequency(ctx context.Context) (*uint32, error) {
	return f.frequency, f.err
}

func (f *fakeAccessPointRaw) GetPairwiseCiphers(ctx context.Context) ([]string, error) {
	return f.pairwiseCiphers, f.err
}

func (f *fakeAccessPointRaw) GetGroupCipher(ctx context.Context) (*string, error) {
	return f.groupCipher, f.err
}

func (f *fakeAccessPointRaw) GetProperties(ctx context.Context) (*iwdbus.AccessPointProperties, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.props != nil {
		return f.props, nil
	}
	return &iwdbus.AccessPointProperties{Started: f.started, Scanning: f.scanning}, nil
}

func (f *fakeAccessPointRaw) Start(ctx context.Context, ssid, psk string) error {
	f.mu.Lock()
	f.startedSSID = ssid
	f.startedPSK = psk
	f.mu.Unlock()
	return f.err
}

func (f *fakeAccessPointRaw) StartProfile(ctx context.Context, ssid string) error {
	f.mu.Lock()
	f.profileSSID = ssid
	f.mu.Unlock()
	return f.err
}

func (f *fakeAccessPointRaw) Stop(ctx context.Context) error {
	f.mu.Lock()
	f.stopCalled = true
	f.mu.Unlock()
	return f.err
}

func (f *fakeAccessPointRaw) Scan(ctx context.Context) error {
	f.mu.Lock()
	f.scanCalled = true
	f.mu.Unlock()
	return f.err
}

func (f *fakeAccessPointRaw) GetOrderedNetworks(ctx context.Context) ([]iwdbus.AccessPointOrderedNetwork, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ordered, nil
}

func (f *fakeAccessPointRaw) SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.AccessPointPropertiesChanged)) (iwdbus.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	if fn != nil && f.subEvent != nil {
		fn(*f.subEvent)
	}
	return func() error { return nil }, nil
}

func (f *fakeAccessPointRaw) SubscribeStartedChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	if fn != nil && f.subEvent != nil {
		if v, ok := f.subEvent.Changed["Started"]; ok {
			if b, ok := v.Value().(bool); ok {
				fn(b)
			}
		}
	}
	return func() error { return nil }, nil
}

func (f *fakeAccessPointRaw) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	if fn != nil && f.subEvent != nil {
		if v, ok := f.subEvent.Changed["Scanning"]; ok {
			if b, ok := v.Value().(bool); ok {
				fn(b)
			}
		}
	}
	return func() error { return nil }, nil
}

func (f *fakeAccessPointRaw) startArgs() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startedSSID, f.startedPSK
}

func (f *fakeAccessPointRaw) profileArg() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.profileSSID
}

func (f *fakeAccessPointRaw) stopWasCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopCalled
}

func (f *fakeAccessPointRaw) scanWasCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.scanCalled
}

// newTestAccessPoint returns an *AccessPoint backed by a concurrency-safe fake
// that reports a running AP.
func newTestAccessPoint(t *testing.T) *AccessPoint {
	t.Helper()
	name := "MyAP"
	freq := uint32(5180)
	a := NewAccessPoint(&fakeAccessPointRaw{started: true, name: &name, frequency: &freq})
	require.NotNil(t, a)
	return a
}
