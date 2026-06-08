// Package testutil provides core-layer test fakes and embeddable stubs.
package testutil

import (
	"context"

	"github.com/chrispypip/spiderw/internal/core"
)

// UnimplementedCoreDaemon is a test-only embeddable stub for fakes that
// implement core daemon interfaces.
//
// Embed this type in a fake when the fake should only provide the daemon
// behavior required by a specific test. Methods on this type intentionally
// panic: a panic means the test called a method that its fake did not explicitly
// implement, which is usually a sign that the test fixture needs to be updated.
type UnimplementedCoreDaemon struct{}

// Adapters panics when a test fake does not implement adapter enumeration.
func (UnimplementedCoreDaemon) Adapters(ctx context.Context) ([]core.AdapterRef, error) {
	panic("fakeCoreDaemon.Adapters not implemented")
}
