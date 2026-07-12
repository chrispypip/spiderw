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

// Devices panics when a test fake does not implement device enumeration.
func (UnimplementedCoreDaemon) Devices(ctx context.Context) ([]core.DeviceRef, error) {
	panic("fakeCoreDaemon.Devices not implemented")
}

// Stations panics when a test fake does not implement station enumeration.
func (UnimplementedCoreDaemon) Stations(ctx context.Context) ([]core.StationRef, error) {
	panic("fakeCoreDaemon.Stations not implemented")
}

func (UnimplementedCoreDaemon) AccessPoints(ctx context.Context) ([]core.AccessPointRef, error) {
	panic("fakeCoreDaemon.AccessPoints not implemented")
}

// BasicServiceSets panics when a test fake does not implement BSS enumeration.
func (UnimplementedCoreDaemon) BasicServiceSets(ctx context.Context) ([]core.BasicServiceSetRef, error) {
	panic("fakeCoreDaemon.BasicServiceSets not implemented")
}

// Networks panics when a test fake does not implement network enumeration.
func (UnimplementedCoreDaemon) Networks(ctx context.Context) ([]core.NetworkRef, error) {
	panic("fakeCoreDaemon.Networks not implemented")
}

// KnownNetworks panics when a test fake does not implement known-network
// enumeration.
func (UnimplementedCoreDaemon) KnownNetworks(ctx context.Context) ([]core.KnownNetworkRef, error) {
	panic("fakeCoreDaemon.KnownNetworks not implemented")
}
