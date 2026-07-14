//go:build unit || race || stress

package core

import (
	"context"
	"sync"

	"github.com/godbus/dbus/v5"
)

const testAgentNetworkPath = "/net/connman/iwd/phy0/wlan0/secure"

// fakeAgentManager is a concurrency-safe agentManagerRaw for core agent tests.
type fakeAgentManager struct {
	mu              sync.Mutex
	registerCalls   []dbus.ObjectPath
	unregisterCalls []dbus.ObjectPath
	unregisterErr   error
}

func (f *fakeAgentManager) RegisterAgent(ctx context.Context, path dbus.ObjectPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registerCalls = append(f.registerCalls, path)
	return nil
}

func (f *fakeAgentManager) UnregisterAgent(ctx context.Context, path dbus.ObjectPath) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unregisterCalls = append(f.unregisterCalls, path)
	return f.unregisterErr
}

func (f *fakeAgentManager) unregisterCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.unregisterCalls)
}
