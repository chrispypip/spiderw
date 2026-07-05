//go:build unit

package cli

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/chrispypip/spiderw"
)

// This file provides in-process fakes for the CLI's client interfaces so command
// behavior (routing, output rendering, error mapping) can be unit-tested without
// a D-Bus connection or the iwd mock.

type fakeClient struct {
	daemon        daemonAPI
	adapters      map[string]adapterAPI      // keyed by Path
	devices       map[string]deviceAPI       // keyed by Path
	stations      map[string]stationAPI      // keyed by Path
	bsses         map[string]bssAPI          // keyed by Path
	networks      map[string]networkAPI      // keyed by Path
	knownNetworks map[string]knownNetworkAPI // keyed by Path
	adapterErr    error                      // returned by Adapter(...)
	deviceErr     error                      // returned by Device(...)
	stationErr    error                      // returned by Station(...)
	bssErr        error                      // returned by BasicServiceSet(...)
	networkErr    error                      // returned by Network(...)
	knownNetErr   error                      // returned by KnownNetwork(...)
	allAdapters   []adapterAPI
	allDevices    []deviceAPI
	allStations   []stationAPI
	allBSSes      []bssAPI
	allNetworks   []networkAPI
	allKnownNets  []knownNetworkAPI
	allAdaptErr   error
	allDeviceErr  error
	allStationErr error
	allBSSErr     error
	allNetErr     error
	allKnownErr   error
	registerErr   error                // returned by RegisterAgent(...)
	registeredCfg *spiderw.AgentConfig // last config passed to RegisterAgent
	agent         *fakeAgent
	closed        bool
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

func (f *fakeClient) Station(_ context.Context, path string) (stationAPI, error) {
	if f.stationErr != nil {
		return nil, f.stationErr
	}
	return f.stations[path], nil
}

func (f *fakeClient) BasicServiceSet(_ context.Context, path string) (bssAPI, error) {
	if f.bssErr != nil {
		return nil, f.bssErr
	}
	return f.bsses[path], nil
}

func (f *fakeClient) Network(_ context.Context, path string) (networkAPI, error) {
	if f.networkErr != nil {
		return nil, f.networkErr
	}
	return f.networks[path], nil
}

func (f *fakeClient) KnownNetwork(_ context.Context, path string) (knownNetworkAPI, error) {
	if f.knownNetErr != nil {
		return nil, f.knownNetErr
	}
	return f.knownNetworks[path], nil
}

func (f *fakeClient) AllAdapters(context.Context) ([]adapterAPI, error) {
	return f.allAdapters, f.allAdaptErr
}

func (f *fakeClient) AllDevices(context.Context) ([]deviceAPI, error) {
	return f.allDevices, f.allDeviceErr
}

func (f *fakeClient) AllStations(context.Context) ([]stationAPI, error) {
	return f.allStations, f.allStationErr
}

func (f *fakeClient) AllBasicServiceSets(context.Context) ([]bssAPI, error) {
	return f.allBSSes, f.allBSSErr
}

func (f *fakeClient) AllNetworks(context.Context) ([]networkAPI, error) {
	return f.allNetworks, f.allNetErr
}

func (f *fakeClient) AllKnownNetworks(context.Context) ([]knownNetworkAPI, error) {
	return f.allKnownNets, f.allKnownErr
}

func (f *fakeClient) RegisterAgent(_ context.Context, cfg spiderw.AgentConfig) (agentAPI, error) {
	if f.registerErr != nil {
		return nil, f.registerErr
	}
	c := cfg
	f.registeredCfg = &c
	if f.agent == nil {
		f.agent = &fakeAgent{}
	}
	return f.agent, nil
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

type fakeAgent struct {
	unregistered  bool
	unregisterErr error
}

func (a *fakeAgent) Unregister(context.Context) error {
	a.unregistered = true
	return a.unregisterErr
}

type fakeDaemon struct {
	info          *spiderw.DaemonInfo
	adapters      []spiderw.AdapterRef
	devices       []spiderw.DeviceRef
	stations      []spiderw.StationRef
	bsses         []spiderw.BasicServiceSetRef
	networks      []spiderw.NetworkRef
	knownNetworks []spiderw.KnownNetworkRef
	err           error
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

func (f *fakeDaemon) Stations(context.Context) ([]spiderw.StationRef, error) {
	return f.stations, f.err
}

func (f *fakeDaemon) BasicServiceSets(context.Context) ([]spiderw.BasicServiceSetRef, error) {
	return f.bsses, f.err
}

func (f *fakeDaemon) Networks(context.Context) ([]spiderw.NetworkRef, error) {
	return f.networks, f.err
}

func (f *fakeDaemon) KnownNetworks(context.Context) ([]spiderw.KnownNetworkRef, error) {
	return f.knownNetworks, f.err
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

type fakeStation struct {
	path            string
	props           *spiderw.StationProperties
	ordered         []spiderw.OrderedNetwork
	scanErr         error
	setAffErr       error
	setAffinitiesTo []string
	scanCalled      bool
	err             error
}

func (f *fakeStation) Path() string { return f.path }

func (f *fakeStation) Properties(context.Context) (*spiderw.StationProperties, error) {
	return f.props, f.err
}

func (f *fakeStation) Affinities(context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.props == nil {
		return nil, nil
	}
	return f.props.Affinities, nil
}

func (f *fakeStation) Scan(context.Context) error {
	f.scanCalled = true
	return f.scanErr
}

func (f *fakeStation) OrderedNetworks(context.Context) ([]spiderw.OrderedNetwork, error) {
	return f.ordered, f.err
}

func (f *fakeStation) SetAffinities(_ context.Context, paths []string) error {
	if f.setAffErr != nil {
		return f.setAffErr
	}
	f.setAffinitiesTo = paths
	return nil
}

func (f *fakeStation) SubscribeScanningChanged(_ context.Context, fn func(bool)) (spiderw.UnsubscribeFunc, error) {
	if f.err != nil {
		return nil, f.err
	}
	// Simulate a completed scan (true then false) so `station scan` (wait mode)
	// returns promptly in unit tests.
	if fn != nil {
		fn(true)
		fn(false)
	}
	return func() error { return nil }, nil
}

type fakeBSS struct {
	path  string
	props *spiderw.BasicServiceSetProperties
	err   error
}

func (f *fakeBSS) Path() string { return f.path }

func (f *fakeBSS) Address(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Address, nil
}

func (f *fakeBSS) Properties(context.Context) (*spiderw.BasicServiceSetProperties, error) {
	return f.props, f.err
}

type fakeNetwork struct {
	path       string
	props      *spiderw.NetworkProperties
	connectErr error
	err        error
}

func (f *fakeNetwork) Path() string { return f.path }

func (f *fakeNetwork) Name(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Name, nil
}

func (f *fakeNetwork) Connected(context.Context) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.props.Connected, nil
}

func (f *fakeNetwork) Device(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Device, nil
}

func (f *fakeNetwork) Type(context.Context) (spiderw.NetworkType, error) {
	if f.err != nil {
		return spiderw.NetworkTypeUnknown, f.err
	}
	return f.props.Type, nil
}

func (f *fakeNetwork) KnownNetwork(context.Context) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.props.KnownNetwork, nil
}

func (f *fakeNetwork) ExtendedServiceSet(context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.props.ExtendedServiceSet, nil
}

func (f *fakeNetwork) Connect(context.Context) error {
	if f.connectErr != nil {
		return f.connectErr
	}
	f.props.Connected = true
	return nil
}

func (f *fakeNetwork) Properties(context.Context) (*spiderw.NetworkProperties, error) {
	return f.props, f.err
}

func (f *fakeNetwork) SubscribeConnectedChanged(context.Context, func(bool)) (spiderw.UnsubscribeFunc, error) {
	return func() error { return nil }, f.err
}

type fakeKnownNetwork struct {
	path      string
	props     *spiderw.KnownNetworkProperties
	forgetErr error
	err       error
}

func (f *fakeKnownNetwork) Path() string { return f.path }

func (f *fakeKnownNetwork) Name(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.props.Name, nil
}

func (f *fakeKnownNetwork) Type(context.Context) (spiderw.NetworkType, error) {
	if f.err != nil {
		return spiderw.NetworkTypeUnknown, f.err
	}
	return f.props.Type, nil
}

func (f *fakeKnownNetwork) Hidden(context.Context) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.props.Hidden, nil
}

func (f *fakeKnownNetwork) LastConnectedTime(context.Context) (*string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.props.LastConnectedTime, nil
}

func (f *fakeKnownNetwork) AutoConnect(context.Context) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.props.AutoConnect, nil
}

func (f *fakeKnownNetwork) SetAutoConnect(_ context.Context, autoConnect bool) error {
	if f.err != nil {
		return f.err
	}
	f.props.AutoConnect = autoConnect
	return nil
}

func (f *fakeKnownNetwork) Forget(context.Context) error {
	return f.forgetErr
}

func (f *fakeKnownNetwork) Properties(context.Context) (*spiderw.KnownNetworkProperties, error) {
	return f.props, f.err
}

func (f *fakeKnownNetwork) SubscribeAutoConnectChanged(context.Context, func(bool)) (spiderw.UnsubscribeFunc, error) {
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

// driveConnect drives the CLI with a stdin source and a passphrase-prompt
// override, for exercising the secured-connect flow. A nil prompt leaves the
// default (terminal) prompt in place; an empty stdin leaves the default.
func driveConnect(fake clientAPI, stdin string, prompt func(string) (string, error), args ...string) (string, int) {
	var buf bytes.Buffer
	app := &App{
		Stdout: &buf,
		Stderr: &buf,
		NewClient: func(context.Context, spiderw.Bus) (clientAPI, error) {
			return fake, nil
		},
		PromptPassphrase: prompt,
	}
	if stdin != "" {
		app.Stdin = strings.NewReader(stdin)
	}
	code := runApp(app, args)
	return buf.String(), code
}
