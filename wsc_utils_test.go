//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync"
)

// fakeCoreSimpleConfig is a concurrency-safe core.SimpleConfigurationIface for
// public WSC tests. Configured error/PIN fields are set at construction and only
// read; recorded calls/PIN are mutex-guarded so race and stress tests can hammer
// it.
type fakeCoreSimpleConfig struct {
	pushErr   error
	genPin    string
	genErr    error
	startErr  error
	cancelErr error

	mu         sync.Mutex
	calls      []string
	startedPin string
}

func (f *fakeCoreSimpleConfig) PushButton(ctx context.Context) error {
	f.record("PushButton")
	return f.pushErr
}

func (f *fakeCoreSimpleConfig) GeneratePin(ctx context.Context) (string, error) {
	f.record("GeneratePin")
	return f.genPin, f.genErr
}

func (f *fakeCoreSimpleConfig) StartPin(ctx context.Context, pin string) error {
	f.mu.Lock()
	f.calls = append(f.calls, "StartPin")
	f.startedPin = pin
	f.mu.Unlock()
	return f.startErr
}

func (f *fakeCoreSimpleConfig) Cancel(ctx context.Context) error {
	f.record("Cancel")
	return f.cancelErr
}

func (f *fakeCoreSimpleConfig) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

// callList returns a copy of the recorded method calls.
func (f *fakeCoreSimpleConfig) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// pinStarted returns the PIN last passed to StartPin.
func (f *fakeCoreSimpleConfig) pinStarted() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startedPin
}
