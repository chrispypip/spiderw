//go:build unit

package cli

import (
	"bytes"
	"context"
	"slices"

	"github.com/chrispypip/spiderw"
)

// This file provides in-process fakes for the CLI's client interfaces so command
// behavior (routing, output rendering, error mapping) can be unit-tested without
// a D-Bus connection or the iwd mock.

type fakeClient struct {
	daemon       daemonAPI
	adapters     map[string]adapterAPI // keyed by Path
	devices      map[string]deviceAPI  // keyed by Path
	adapterErr   error                 // returned by Adapter(...)
	deviceErr    error                 // returned by Device(...)
	allAdapters  []adapterAPI
	allDevices   []deviceAPI
	allAdaptErr  error
	allDeviceErr error
	closed       bool
}

func (f *fakeClient) Daemon() daemonAPI { return f.daemon }

func (f *fakeClient) Adapter(_ context.Context, path string) (adapterAPI, error) {
	if f.adapterErr != nil {
		return nil, f.adapterErr
	}
	return f.adapters[path], nil
}

func (f *fakeClient) Device(_ context.Context, path string) (deviceAPI, error) {
	if f.deviceErr != nil {
		return nil, f.deviceErr
	}
	return f.devices[path], nil
}

func (f *fakeClient) AllAdapters(context.Context) ([]adapterAPI, error) {
	return f.allAdapters, f.allAdaptErr
}

func (f *fakeClient) AllDevices(context.Context) ([]deviceAPI, error) {
	return f.allDevices, f.allDeviceErr
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

type fakeDaemon struct {
	info     *spiderw.DaemonInfo
	adapters []spiderw.AdapterRef
	devices  []spiderw.DeviceRef
	err      error
}

func (f *fakeDaemon) Info(context.Context) (*spiderw.DaemonInfo, error) {
	return f.info, f.err
}

func (f *fakeDaemon) Version(context.Context) (string, error) {
	if f.err != nil || f.info == nil {
		return "", f.err
	}
	return f.info.Version, nil
}

func (f *fakeDaemon) StateDirectory(context.Context) (string, error) {
	if f.err != nil || f.info == nil {
		return "", f.err
	}
	return f.info.StateDirectory, nil
}

func (f *fakeDaemon) NetworkConfigurationEnabled(context.Context) (bool, error) {
	if f.err != nil || f.info == nil {
		return false, f.err
	}
	return f.info.NetworkConfigurationEnabled, nil
}

func (f *fakeDaemon) Adapters(context.Context) ([]spiderw.AdapterRef, error) {
	return f.adapters, f.err
}

func (f *fakeDaemon) Devices(context.Context) ([]spiderw.DeviceRef, error) {
	return f.devices, f.err
}

type fakeAdapter struct {
	path  string
	props *spiderw.AdapterProperties
	err   error // returned by accessors when set
}

func (f *fakeAdapter) Path() string { return f.path }

func (f *fakeAdapter) Powered(context.Context) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.props.Powered, nil
}

func (f *fakeAdapter) SetPowered(_ context.Context, powered bool) error {
	if f.err != nil {
		return f.err
	}
	f.props.Powered = powered
	return nil
}

func (f *fakeAdapter) Name(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Name, nil
}

func (f *fakeAdapter) Model(context.Context) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.props.Model, nil
}

func (f *fakeAdapter) Vendor(context.Context) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.props.Vendor, nil
}

func (f *fakeAdapter) SupportedModes(context.Context) ([]spiderw.Mode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.props.SupportedModes, nil
}

func (f *fakeAdapter) supports(mode spiderw.Mode) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return slices.Contains(f.props.SupportedModes, mode), nil
}

func (f *fakeAdapter) SupportsMode(_ context.Context, mode spiderw.Mode) (bool, error) {
	return f.supports(mode)
}

func (f *fakeAdapter) SupportsStation(context.Context) (bool, error) {
	return f.supports(spiderw.ModeStation)
}

func (f *fakeAdapter) SupportsAP(context.Context) (bool, error) { return f.supports(spiderw.ModeAP) }

func (f *fakeAdapter) SupportsAdHoc(context.Context) (bool, error) {
	return f.supports(spiderw.ModeAdHoc)
}

func (f *fakeAdapter) Properties(context.Context) (*spiderw.AdapterProperties, error) {
	return f.props, f.err
}

func (f *fakeAdapter) SubscribePoweredChanged(context.Context, func(bool)) (spiderw.UnsubscribeFunc, error) {
	return func() error { return nil }, f.err
}

type fakeDevice struct {
	path  string
	props *spiderw.DeviceProperties
	err   error
}

func (f *fakeDevice) Path() string { return f.path }

func (f *fakeDevice) Powered(context.Context) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.props.Powered, nil
}

func (f *fakeDevice) SetPowered(_ context.Context, powered bool) error {
	if f.err != nil {
		return f.err
	}
	f.props.Powered = powered
	return nil
}

func (f *fakeDevice) Name(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Name, nil
}

func (f *fakeDevice) Address(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Address, nil
}

func (f *fakeDevice) Mode(context.Context) (spiderw.Mode, error) {
	if f.err != nil {
		return spiderw.ModeUnknown, f.err
	}
	return f.props.Mode, nil
}

func (f *fakeDevice) SetMode(_ context.Context, mode spiderw.Mode) error {
	if f.err != nil {
		return f.err
	}
	f.props.Mode = mode
	return nil
}

func (f *fakeDevice) Adapter(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Adapter, nil
}

func (f *fakeDevice) Properties(context.Context) (*spiderw.DeviceProperties, error) {
	return f.props, f.err
}

func (f *fakeDevice) SubscribePoweredChanged(context.Context, func(bool)) (spiderw.UnsubscribeFunc, error) {
	return func() error { return nil }, f.err
}

func (f *fakeDevice) SubscribeModeChanged(context.Context, func(spiderw.Mode)) (spiderw.UnsubscribeFunc, error) {
	return func() error { return nil }, f.err
}

// driveCLI runs the CLI in-process against a faked client, capturing combined
// stdout+stderr and returning it with the process exit code. clientErr, when
// non-nil, simulates a client-construction failure.
func driveCLI(fake clientAPI, clientErr error, jsonOut bool, args ...string) (string, int) {
	var buf bytes.Buffer
	app := &App{
		Stdout: &buf,
		Stderr: &buf,
		Output: outputConfig{JSON: jsonOut},
		NewClient: func(context.Context, spiderw.Bus) (clientAPI, error) {
			if clientErr != nil {
				return nil, clientErr
			}
			return fake, nil
		},
	}
	code := runApp(app, args)
	return buf.String(), code
}
