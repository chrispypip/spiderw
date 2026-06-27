package connect

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/iwdbus"
)

var (
	connectSystemBusFn    = dbus.ConnectSystemBus
	connectSessionBusFn   = dbus.ConnectSessionBus
	newIwdDaemonFn        = iwdbus.NewDaemon
	newIwdAdapterFn       = iwdbus.NewAdapter
	newIwdDeviceFn        = iwdbus.NewDevice
	newIwdBSSFn           = iwdbus.NewBasicServiceSet
	newIwdNetworkFn       = iwdbus.NewNetwork
	newIwdKnownNetworkFn  = iwdbus.NewKnownNetwork
	newIwdAgentManagerFn  = iwdbus.NewAgentManager
	exportAgentFn         = iwdbus.ExportAgent
	newCoreDaemonFn       = func(raw *iwdbus.Daemon) *core.Daemon { return core.NewDaemon(raw) }
	newCoreAdapterFn      = func(raw *iwdbus.Adapter) *core.Adapter { return core.NewAdapter(raw) }
	newCoreDeviceFn       = func(raw *iwdbus.Device) *core.Device { return core.NewDevice(raw) }
	newCoreBSSFn          = func(raw *iwdbus.BasicServiceSet) *core.BasicServiceSet { return core.NewBasicServiceSet(raw) }
	newCoreNetworkFn      = func(raw *iwdbus.Network) *core.Network { return core.NewNetwork(raw) }
	newCoreKnownNetworkFn = func(raw *iwdbus.KnownNetwork) *core.KnownNetwork {
		return core.NewKnownNetwork(raw)
	}
	closeConnFn = func(c *dbus.Conn) error { return c.Close() }
)

func newConn(ctx context.Context, system bool) (*Wiring, error) {
	const op = "Wiring.newConn"

	var conn *dbus.Conn
	var err error

	if system {
		conn, err = connectSystemBusFn()
	} else {
		conn, err = connectSessionBusFn()
	}
	if err != nil {
		return nil, core.WrapDaemonUnavailable("Wiring.newConn", "failed to connect to bus", err)
	}

	var cleanupOnce sync.Once
	var cleanupErr error
	cleanup := func() error {
		cleanupOnce.Do(func() {
			cleanupErr = closeConnFn(conn)
		})
		return cleanupErr
	}

	iwdDaemon, err := newIwdDaemonFn(ctx, conn)
	if err != nil {
		if err2 := cleanup(); err2 != nil {
			return nil, errors.Join(err, core.WrapDaemonUnavailable(op, "failed iwd daemon cleanup", err2))
		}
		return nil, core.WrapDaemonUnavailable(op, "failed to get iwd daemon", err)
	}
	if iwdDaemon == nil {
		if err := cleanup(); err != nil {
			return nil, errors.Join(err, core.WrapDaemonUnavailable(op, "failed iwd daemon cleanup", err))
		}
		return nil, core.WrapDaemonUnavailable(op, "failed to get iwd daemon", fmt.Errorf("iwd daemon interface not available"))
	}
	coreDaemon := newCoreDaemonFn(iwdDaemon)
	if coreDaemon == nil {
		if err := cleanup(); err != nil {
			return nil, errors.Join(err, core.WrapDaemonUnavailable("NewDaemon", "daemon unavailable", fmt.Errorf("failed core daemon cleanup")))
		}
		return nil, core.WrapDaemonUnavailable(op, "daemon unavailable", fmt.Errorf("core daemon interface not available"))
	}

	return &Wiring{
		Conn:    conn,
		Daemon:  coreDaemon,
		Cleanup: cleanup,
	}, nil
}

// Wiring bundles the constructed D-Bus connection and core objects for the public layer.
type Wiring struct {
	// Conn is the owned D-Bus connection used to construct iwd objects.
	Conn *dbus.Conn

	// Daemon is the core-layer daemon wrapper built from Conn.
	Daemon core.DaemonIface

	// Cleanup releases resources owned by this Wiring.
	Cleanup func() error

	// AdapterFactory optionally overrides adapter construction for tests.
	AdapterFactory func(ctx context.Context, path string) (core.AdapterIface, error)

	// DeviceFactory optionally overrides device construction for tests.
	DeviceFactory func(ctx context.Context, path string) (core.DeviceIface, error)

	// BasicServiceSetFactory optionally overrides BSS construction for tests.
	BasicServiceSetFactory func(ctx context.Context, path string) (core.BasicServiceSetIface, error)

	// NetworkFactory optionally overrides network construction for tests.
	NetworkFactory func(ctx context.Context, path string) (core.NetworkIface, error)

	// KnownNetworkFactory optionally overrides known-network construction for
	// tests.
	KnownNetworkFactory func(ctx context.Context, path string) (core.KnownNetworkIface, error)

	// AgentFactory optionally overrides agent registration for tests.
	AgentFactory func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error)
}

// agentObjectPath is the D-Bus object path the credentials agent is exported at.
// It lives in spiderw's own path namespace (the connection owns a unique bus
// name), not under iwd's tree.
const agentObjectPath = dbus.ObjectPath("/spiderw/agent")

// NewAdapter constructs a core adapter wrapper for the given iwd object path.
func (w *Wiring) NewAdapter(ctx context.Context, path string) (core.AdapterIface, error) {
	const op = "NewAdapter"

	if w == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "wiring cannot be nil", core.ErrCore)
	}
	if path == "" {
		return nil, core.WrapInvalidArgument(core.ResourceAdapter, op, "adapter path cannot be empty", core.ErrCore)
	}
	if path[0] != '/' {
		return nil, core.WrapInvalidArgument(core.ResourceAdapter, op, "adapter path must be absolute", core.ErrCore)
	}
	if w.AdapterFactory != nil {
		return w.AdapterFactory(ctx, path)
	}
	if w.Conn == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "D-Bus conn cannot be nil", core.ErrCore)
	}

	iwdAdapter, err := newIwdAdapterFn(ctx, w.Conn, dbus.ObjectPath(path))
	if err != nil {
		return nil, core.WrapAdapterUnavailable("NewAdapter", "adapter unavailable", err)
	}
	if iwdAdapter == nil {
		return nil, core.WrapAdapterUnavailable("NewAdapter", "adapter unavailable", iwdbus.WrapIntrospection(path, fmt.Errorf("iwd adapter interface not available")))
	}

	coreAdapter := newCoreAdapterFn(iwdAdapter)
	if coreAdapter == nil {
		return nil, core.WrapAdapterUnavailable("NewAdapter", "adapter unavailable", fmt.Errorf("core adapter interface not available"))
	}
	return coreAdapter, nil
}

// NewDevice constructs a core device wrapper for the given iwd object path.
func (w *Wiring) NewDevice(ctx context.Context, path string) (core.DeviceIface, error) {
	const op = "NewDevice"

	if w == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "wiring cannot be nil", core.ErrCore)
	}
	if path == "" {
		return nil, core.WrapInvalidArgument(core.ResourceDevice, op, "device path cannot be empty", core.ErrCore)
	}
	if path[0] != '/' {
		return nil, core.WrapInvalidArgument(core.ResourceDevice, op, "device path must be absolute", core.ErrCore)
	}
	if w.DeviceFactory != nil {
		return w.DeviceFactory(ctx, path)
	}
	if w.Conn == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "D-Bus conn cannot be nil", core.ErrCore)
	}

	iwdDevice, err := newIwdDeviceFn(ctx, w.Conn, dbus.ObjectPath(path))
	if err != nil {
		return nil, core.WrapDeviceUnavailable(op, "device unavailable", err)
	}
	if iwdDevice == nil {
		return nil, core.WrapDeviceUnavailable(op, "device unavailable", iwdbus.WrapIntrospection(path, fmt.Errorf("iwd device interface not available")))
	}

	coreDevice := newCoreDeviceFn(iwdDevice)
	if coreDevice == nil {
		return nil, core.WrapDeviceUnavailable(op, "device unavailable", fmt.Errorf("core device interface not available"))
	}
	return coreDevice, nil
}

// NewBasicServiceSet constructs a core BSS wrapper for the given iwd object path.
func (w *Wiring) NewBasicServiceSet(ctx context.Context, path string) (core.BasicServiceSetIface, error) {
	const op = "NewBasicServiceSet"

	if w == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "wiring cannot be nil", core.ErrCore)
	}
	if path == "" {
		return nil, core.WrapInvalidArgument(core.ResourceBasicServiceSet, op, "basic service set path cannot be empty", core.ErrCore)
	}
	if path[0] != '/' {
		return nil, core.WrapInvalidArgument(core.ResourceBasicServiceSet, op, "basic service set path must be absolute", core.ErrCore)
	}
	if w.BasicServiceSetFactory != nil {
		return w.BasicServiceSetFactory(ctx, path)
	}
	if w.Conn == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "D-Bus conn cannot be nil", core.ErrCore)
	}

	iwdBSS, err := newIwdBSSFn(ctx, w.Conn, dbus.ObjectPath(path))
	if err != nil {
		return nil, core.WrapBasicServiceSetUnavailable(op, "basic service set unavailable", err)
	}
	if iwdBSS == nil {
		return nil, core.WrapBasicServiceSetUnavailable(op, "basic service set unavailable", iwdbus.WrapIntrospection(path, fmt.Errorf("iwd basic service set interface not available")))
	}

	coreBSS := newCoreBSSFn(iwdBSS)
	if coreBSS == nil {
		return nil, core.WrapBasicServiceSetUnavailable(op, "basic service set unavailable", fmt.Errorf("core basic service set interface not available"))
	}
	return coreBSS, nil
}

// NewNetwork constructs a core network wrapper for the given iwd object path.
func (w *Wiring) NewNetwork(ctx context.Context, path string) (core.NetworkIface, error) {
	const op = "NewNetwork"

	if w == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "wiring cannot be nil", core.ErrCore)
	}
	if path == "" {
		return nil, core.WrapInvalidArgument(core.ResourceNetwork, op, "network path cannot be empty", core.ErrCore)
	}
	if path[0] != '/' {
		return nil, core.WrapInvalidArgument(core.ResourceNetwork, op, "network path must be absolute", core.ErrCore)
	}
	if w.NetworkFactory != nil {
		return w.NetworkFactory(ctx, path)
	}
	if w.Conn == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "D-Bus conn cannot be nil", core.ErrCore)
	}

	iwdNetwork, err := newIwdNetworkFn(ctx, w.Conn, dbus.ObjectPath(path))
	if err != nil {
		return nil, core.WrapNetworkUnavailable(op, "network unavailable", err)
	}
	if iwdNetwork == nil {
		return nil, core.WrapNetworkUnavailable(op, "network unavailable", iwdbus.WrapIntrospection(path, fmt.Errorf("iwd network interface not available")))
	}

	coreNetwork := newCoreNetworkFn(iwdNetwork)
	if coreNetwork == nil {
		return nil, core.WrapNetworkUnavailable(op, "network unavailable", fmt.Errorf("core network interface not available"))
	}
	return coreNetwork, nil
}

// NewKnownNetwork constructs a core known-network wrapper for the given iwd
// object path.
func (w *Wiring) NewKnownNetwork(ctx context.Context, path string) (core.KnownNetworkIface, error) {
	const op = "NewKnownNetwork"

	if w == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "wiring cannot be nil", core.ErrCore)
	}
	if path == "" {
		return nil, core.WrapInvalidArgument(core.ResourceKnownNetwork, op, "known network path cannot be empty", core.ErrCore)
	}
	if path[0] != '/' {
		return nil, core.WrapInvalidArgument(core.ResourceKnownNetwork, op, "known network path must be absolute", core.ErrCore)
	}
	if w.KnownNetworkFactory != nil {
		return w.KnownNetworkFactory(ctx, path)
	}
	if w.Conn == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "D-Bus conn cannot be nil", core.ErrCore)
	}

	iwdKnownNetwork, err := newIwdKnownNetworkFn(ctx, w.Conn, dbus.ObjectPath(path))
	if err != nil {
		return nil, core.WrapKnownNetworkUnavailable(op, "known network unavailable", err)
	}
	if iwdKnownNetwork == nil {
		return nil, core.WrapKnownNetworkUnavailable(op, "known network unavailable", iwdbus.WrapIntrospection(path, fmt.Errorf("iwd known network interface not available")))
	}

	coreKnownNetwork := newCoreKnownNetworkFn(iwdKnownNetwork)
	if coreKnownNetwork == nil {
		return nil, core.WrapKnownNetworkUnavailable(op, "known network unavailable", fmt.Errorf("core known network interface not available"))
	}
	return coreKnownNetwork, nil
}

// NewAgent exports a credentials agent built from cc and registers it with iwd,
// returning a handle whose Unregister tears it back down.
//
// Unlike the single-object constructors above, an agent is an object spiderw
// exports and iwd calls into: NewAgent exports the agent at agentObjectPath, then
// calls AgentManager.RegisterAgent. On any failure after export, the object is
// unexported before returning.
func (w *Wiring) NewAgent(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
	const op = "NewAgent"

	if w == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "wiring cannot be nil", core.ErrCore)
	}
	if w.AgentFactory != nil {
		return w.AgentFactory(ctx, cc)
	}
	if w.Conn == nil {
		return nil, core.WrapInvalidState(core.ResourceClient, op, "D-Bus conn cannot be nil", core.ErrCore)
	}

	agent, handler := core.NewAgent(cc)

	unexport, err := exportAgentFn(w.Conn, agentObjectPath, handler)
	if err != nil {
		return nil, core.WrapAgentUnavailable(op, "failed exporting agent object", err)
	}

	manager, err := newIwdAgentManagerFn(ctx, w.Conn)
	if err != nil {
		_ = unexport()
		return nil, core.WrapAgentUnavailable(op, "agent manager unavailable", err)
	}

	if err := manager.RegisterAgent(ctx, agentObjectPath); err != nil {
		_ = unexport()
		return nil, core.WrapAgentUnavailable(op, "failed registering agent", err)
	}

	agent.Bind(manager, agentObjectPath, unexport)
	return agent, nil
}
