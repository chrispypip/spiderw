//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync/atomic"
)

// fakeCoreSignalLevelAgent is a concurrency-safe core.SignalLevelAgentIface for
// public signal-level agent tests.
type fakeCoreSignalLevelAgent struct {
	unregisterErr error
	unregisters   atomic.Int32
}

func (f *fakeCoreSignalLevelAgent) Unregister(ctx context.Context) error {
	f.unregisters.Add(1)
	return f.unregisterErr
}
