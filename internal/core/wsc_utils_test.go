//go:build unit || race || stress

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeSimpleConfigRaw is a concurrency-safe simpleConfigurationRaw for core WSC
// tests. Configured error/PIN fields are set at construction and only read; the
// recorded calls/PIN are mutex-guarded so race and stress tests can hammer it.
type fakeSimpleConfigRaw struct {
	pushErr   error
	genPin    string
	genErr    error
	startErr  error
	cancelErr error

	mu         sync.Mutex
	calls      []string
	startedPin string
}

func (f *fakeSimpleConfigRaw) PushButton(ctx context.Context) error {
	f.record("PushButton")
	return f.pushErr
}

func (f *fakeSimpleConfigRaw) GeneratePin(ctx context.Context) (string, error) {
	f.record("GeneratePin")
	return f.genPin, f.genErr
}

func (f *fakeSimpleConfigRaw) StartPin(ctx context.Context, pin string) error {
	f.mu.Lock()
	f.calls = append(f.calls, "StartPin")
	f.startedPin = pin
	f.mu.Unlock()
	return f.startErr
}

func (f *fakeSimpleConfigRaw) Cancel(ctx context.Context) error {
	f.record("Cancel")
	return f.cancelErr
}

func (f *fakeSimpleConfigRaw) record(name string) {
	f.mu.Lock()
	f.calls = append(f.calls, name)
	f.mu.Unlock()
}

// callList returns a copy of the recorded method calls.
func (f *fakeSimpleConfigRaw) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// pinStarted returns the PIN last passed to StartPin.
func (f *fakeSimpleConfigRaw) pinStarted() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startedPin
}

// newTestSimpleConfiguration returns a *SimpleConfiguration backed by a
// concurrency-safe fake that generates a fixed PIN and succeeds by default.
func newTestSimpleConfiguration(t *testing.T) *SimpleConfiguration {
	t.Helper()
	c := NewSimpleConfiguration(&fakeSimpleConfigRaw{genPin: "12345670"})
	require.NotNil(t, c)
	return c
}
