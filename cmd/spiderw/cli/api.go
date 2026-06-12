package cli

import (
	"context"

	"github.com/chrispypip/spiderw"
)

// The CLI depends on these narrow interfaces rather than the concrete
// *spiderw.Client / *spiderw.Daemon / *spiderw.Adapter / *spiderw.Device types,
// so commands can be driven in unit tests against fakes without a D-Bus
// connection. The public types satisfy the leaf interfaces directly; only the
// client level needs a wrapper (realClient) because its methods return concrete
// pointers that Go will not implicitly treat as interface return types.

type clientAPI interface {
	Daemon() daemonAPI
	Adapter(ctx context.Context, path string) (adapterAPI, error)
	Device(ctx context.Context, path string) (deviceAPI, error)
	AllAdapters(ctx context.Context) ([]adapterAPI, error)
	AllDevices(ctx context.Context) ([]deviceAPI, error)
	Close() error
}

type daemonAPI interface {
	Info(ctx context.Context) (*spiderw.DaemonInfo, error)
	Version(ctx context.Context) (string, error)
	StateDirectory(ctx context.Context) (string, error)
	NetworkConfigurationEnabled(ctx context.Context) (bool, error)
	Adapters(ctx context.Context) ([]spiderw.AdapterRef, error)
	Devices(ctx context.Context) ([]spiderw.DeviceRef, error)
}

type adapterAPI interface {
	Path() string
	Powered(ctx context.Context) (bool, error)
	SetPowered(ctx context.Context, powered bool) error
	Name(ctx context.Context) (string, error)
	Model(ctx context.Context) (*string, error)
	Vendor(ctx context.Context) (*string, error)
	SupportedModes(ctx context.Context) ([]spiderw.Mode, error)
	SupportsMode(ctx context.Context, mode spiderw.Mode) (bool, error)
	SupportsStation(ctx context.Context) (bool, error)
	SupportsAP(ctx context.Context) (bool, error)
	SupportsAdHoc(ctx context.Context) (bool, error)
	Properties(ctx context.Context) (*spiderw.AdapterProperties, error)
	SubscribePoweredChanged(ctx context.Context, fn func(bool)) (spiderw.UnsubscribeFunc, error)
}

type deviceAPI interface {
	Path() string
	Powered(ctx context.Context) (bool, error)
	SetPowered(ctx context.Context, powered bool) error
	Name(ctx context.Context) (string, error)
	Address(ctx context.Context) (string, error)
	Mode(ctx context.Context) (spiderw.Mode, error)
	SetMode(ctx context.Context, mode spiderw.Mode) error
	Adapter(ctx context.Context) (string, error)
	Properties(ctx context.Context) (*spiderw.DeviceProperties, error)
	SubscribePoweredChanged(ctx context.Context, fn func(bool)) (spiderw.UnsubscribeFunc, error)
	SubscribeModeChanged(ctx context.Context, fn func(spiderw.Mode)) (spiderw.UnsubscribeFunc, error)
}

// realClient adapts a concrete *spiderw.Client to clientAPI, converting the
// concrete return types into the interface forms the CLI consumes.
type realClient struct {
	c *spiderw.Client
}

func (r realClient) Daemon() daemonAPI {
	d := r.c.Daemon()
	if d == nil {
		return nil
	}
	return d
}

func (r realClient) Adapter(ctx context.Context, path string) (adapterAPI, error) {
	a, err := r.c.Adapter(ctx, path)
	if a == nil {
		return nil, err
	}
	return a, err
}

func (r realClient) Device(ctx context.Context, path string) (deviceAPI, error) {
	d, err := r.c.Device(ctx, path)
	if d == nil {
		return nil, err
	}
	return d, err
}

func (r realClient) AllAdapters(ctx context.Context) ([]adapterAPI, error) {
	as, err := r.c.AllAdapters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]adapterAPI, 0, len(as))
	for _, a := range as {
		out = append(out, a)
	}
	return out, nil
}

func (r realClient) AllDevices(ctx context.Context) ([]deviceAPI, error) {
	ds, err := r.c.AllDevices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]deviceAPI, 0, len(ds))
	for _, d := range ds {
		out = append(out, d)
	}
	return out, nil
}

func (r realClient) Close() error {
	return r.c.Close()
}

// defaultClientFactory connects to iwd over D-Bus and wraps the resulting
// client in the clientAPI interface.
func defaultClientFactory(ctx context.Context, bus spiderw.Bus) (clientAPI, error) {
	c, err := spiderw.NewClient(ctx, bus)
	if err != nil {
		return nil, err
	}
	return realClient{c: c}, nil
}
