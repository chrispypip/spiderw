//go:build unit || race || stress

package core

import (
	"context"
	"sync"

	"github.com/godbus/dbus/v5"
)

// fakeSignalLevelRegistrar is a concurrency-safe signalLevelRegistrarRaw for core
// signal-level agent tests.
type fakeSignalLevelRegistrar struct {
	mu              sync.Mutex
	unregisterCalls []dbus.ObjectPath
	unregisterErr   error
}

func (f *fakeSignalLevelRegistrar) UnregisterSignalLevelAgent(ctx context.Context, path dbus.ObjectPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unregisterCalls = append(f.unregisterCalls, path)
	return f.unregisterErr
}

func (f *fakeSignalLevelRegistrar) unregisterCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.unregisterCalls)
}
