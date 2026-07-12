//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync"

	"github.com/chrispypip/spiderw/internal/core"
)

// fakeCoreAccessPoint is a concurrency-safe core.AccessPointIface for public
// AccessPoint tests. Configured return fields are set at construction and only
// read; recorded call arguments are mutex-guarded.
type fakeCoreAccessPoint struct {
	started         bool
	scanning        bool
	ssid            *string // returned by Name (iwd's Name property = SSID)
	frequency       *uint32
	pairwiseCiphers []string
	groupCipher     *string
	ordered         []core.AccessPointOrderedNetwork
	props           *core.AccessPointProperties
	err             error
	subEvent        *core.AccessPointPropertiesChanged

	mu          sync.Mutex
	calls       []string
	startedSSID string
	startedPSK  string
	profileSSID string
}

func (f *fakeCoreAccessPoint) Started(ctx context.Context) (bool, error)  { return f.started, f.err }
func (f *fakeCoreAccessPoint) Scanning(ctx context.Context) (bool, error) { return f.scanning, f.err }
func (f *fakeCoreAccessPoint) Name(ctx context.Context) (*string, error)  { return f.ssid, f.err }

func (f *fakeCoreAccessPoint) Frequency(ctx context.Context) (*uint32, error) {
	return f.frequency, f.err
}

func (f *fakeCoreAccessPoint) PairwiseCiphers(ctx context.Context) ([]string, error) {
	return f.pairwiseCiphers, f.err
}

func (f *fakeCoreAccessPoint) GroupCipher(ctx context.Context) (*string, error) {
	return f.groupCipher, f.err
}

func (f *fakeCoreAccessPoint) Properties(ctx context.Context) (*core.AccessPointProperties, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.props != nil {
		return f.props, nil
	}
	return &core.AccessPointProperties{Started: f.started, Scanning: f.scanning}, nil
}

func (f *fakeCoreAccessPoint) Start(ctx context.Context, ssid, psk string) error {
	f.mu.Lock()
	f.calls = append(f.calls, "Start")
	f.startedSSID = ssid
	f.startedPSK = psk
	f.mu.Unlock()
	return f.err
}

func (f *fakeCoreAccessPoint) StartProfile(ctx context.Context, ssid string) error {
	f.mu.Lock()
	f.calls = append(f.calls, "StartProfile")
	f.profileSSID = ssid
	f.mu.Unlock()
	return f.err
}

func (f *fakeCoreAccessPoint) Stop(ctx context.Context) error {
	f.record("Stop")
	return f.err
}

func (f *fakeCoreAccessPoint) Scan(ctx context.Context) error {
	f.record("Scan")
	return f.err
}

func (f *fakeCoreAccessPoint) OrderedNetworks(ctx context.Context) ([]core.AccessPointOrderedNetwork, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ordered, nil
}

func (f *fakeCoreAccessPoint) SubscribePropertiesChanged(ctx context.Context, fn func(core.AccessPointPropertiesChanged)) (core.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	if fn != nil && f.subEvent != nil {
		fn(*f.subEvent)
	}
	return func() error { return nil }, nil
}

func (f *fakeCoreAccessPoint) SubscribeStartedChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	if fn != nil && f.subEvent != nil {
		if v, ok := f.subEvent.Changed["Started"]; ok {
			if b, ok := v.(bool); ok {
				fn(b)
			}
		}
	}
	return func() error { return nil }, nil
}

func (f *fakeCoreAccessPoint) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	if fn != nil && f.subEvent != nil {
		if v, ok := f.subEvent.Changed["Scanning"]; ok {
			if b, ok := v.(bool); ok {
				fn(b)
			}
		}
	}
	return func() error { return nil }, nil
}

func (f *fakeCoreAccessPoint) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

func (f *fakeCoreAccessPoint) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeCoreAccessPoint) startArgs() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startedSSID, f.startedPSK
}

func (f *fakeCoreAccessPoint) profileArg() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.profileSSID
}
