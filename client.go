package spiderw

import (
	"context"
	"errors"
	"sync"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/logging"
)

var (
	systemConnectFn  = connect.System
	sessionConnectFn = connect.Session
)

// Client is the root of the public spiderw API.
//
// A Client owns one D-Bus connection and the wiring derived from it. Call Close
// when the client is no longer needed.
type Client struct {
	daemon    *Daemon
	wire      *connect.Wiring
	cleanup   func() error
	agent     *Agent
	closeMu   sync.RWMutex
	closeOnce sync.Once
	closed    bool
	closeErr  error
}

func validateClientWiring(op string, w *connect.Wiring) error {
	if w == nil {
		return wrapPublicError(op, errors.New("nil wiring"))
	}
	if w.Conn == nil {
		return wrapPublicError(op, errors.New("nil connection in wiring"))
	}
	if w.Daemon == nil {
		return wrapPublicError(op, errors.New("nil daemon in wiring"))
	}
	if w.Cleanup == nil {
		return wrapPublicError(op, errors.New("nil cleanup in wiring"))
	}
	return nil
}

// Bus selects which D-Bus message bus a Client connects to.
//
// Bus is a defined boolean type, so call sites may pass the named constants
// (SystemBus / SessionBus) for clarity or a bare bool literal interchangeably.
// The zero value is SystemBus.
type Bus bool

const (
	// SystemBus connects to the system bus. This is the default, and is what
	// real iwd deployments use.
	SystemBus Bus = false

	// SessionBus connects to the session bus, which is primarily useful for
	// tests and mocks.
	SessionBus Bus = true
)

// String returns "system" or "session".
func (b Bus) String() string {
	if b == SessionBus {
		return "session"
	}
	return "system"
}

// NewClient connects to iwd over D-Bus and initializes a Client.
//
// By default NewClient connects to the system bus (SystemBus), which is what
// real iwd deployments use. Pass SessionBus to connect to the session bus
// instead, which is primarily useful for tests and mocks.
func NewClient(ctx context.Context, bus Bus) (*Client, error) {
	const op = "NewClient"
	log := logging.FromContext(ctx)

	log.Debug(ctx, "initializing spiderw client", "bus", bus.String())

	var wire *connect.Wiring
	var err error
	if bus == SessionBus {
		log.Debug(ctx, "connecting via session bus")
		wire, err = sessionConnectFn(ctx)
	} else {
		log.Debug(ctx, "connecting via system bus")
		wire, err = systemConnectFn(ctx)
	}
	if err != nil {
		log.Error(ctx, "dbus wiring failed", "err", err)
		return nil, wrapPublicError(op, err)
	}
	if err := validateClientWiring(op, wire); err != nil {
		log.Error(ctx, "dbus wiring invalid", "err", err)
		return nil, err
	}

	log.Debug(ctx, "wiring established; creating daemon")

	c := &Client{
		daemon:  newDaemon(wire.Daemon),
		wire:    wire,
		cleanup: wire.Cleanup,
	}

	log.Debug(ctx, "client initialized successfully")
	return c, nil
}

// newClientFromWiring constructs a Client from pre-built internal wiring.
//
// The returned Client owns the supplied wiring and calls its Cleanup function
// from Close. This constructor is primarily useful for tests and advanced
// internal integration points.
func newClientFromWiring(w *connect.Wiring) (*Client, error) {
	const op = "newClientFromWiring"
	if err := validateClientWiring(op, w); err != nil {
		return nil, err
	}

	return &Client{
		daemon:  newDaemon(w.Daemon),
		wire:    w,
		cleanup: w.Cleanup,
	}, nil
}

// Close releases resources owned by the client.
//
// Close is idempotent. After Close, Daemon returns nil and Adapter returns an
// invalid-state error.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}

	c.closeOnce.Do(func() {
		c.closeMu.Lock()
		c.closed = true
		c.daemon = nil
		agent := c.agent
		c.agent = nil
		cleanup := c.cleanup
		// Release the lock before calling out to Unregister: the agent's clear
		// callback re-acquires closeMu, and cleanup may block on D-Bus I/O.
		c.closeMu.Unlock()

		// Best-effort: unregister a live agent so iwd drops its reference before
		// the connection closes. Unregister is idempotent.
		if agent != nil {
			_ = agent.Unregister(context.Background())
		}

		if cleanup == nil {
			return
		}

		err := cleanup()
		if err == nil {
			return
		}

		if _, ok := errors.AsType[*Error](err); ok {
			c.closeErr = err
			return
		}
		c.closeErr = wrapPublicError("Client.Close", err)
	})
	return c.closeErr
}

// Daemon returns the singleton iwd daemon wrapper for this client.
//
// Daemon returns nil after the client has been closed.
func (c *Client) Daemon() *Daemon {
	if c == nil {
		return nil
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		return nil
	}
	return c.daemon
}

// RegisterAgent registers a credentials agent so iwd can request credentials
// when connecting to a secured network that is not already known. Without a
// registered agent, Network.Connect on such a network fails (ErrUnavailable, with
// iwd's ErrNoAgent in the chain).
//
// At least one request callback in cfg must be set, or RegisterAgent returns an
// invalid-argument error. A Client owns a single agent: RegisterAgent returns an
// invalid-state error if one is already registered, so Unregister the previous
// agent first. The returned Agent is also unregistered automatically on Close.
func (c *Client) RegisterAgent(ctx context.Context, cfg AgentConfig) (*Agent, error) {
	const op = "Client.RegisterAgent"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	cc := cfg.toCore()
	if err := cc.Validate(op); err != nil {
		log.Error(ctx, "invalid agent config", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	if c.agent != nil {
		log.Error(ctx, "agent already registered", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceAgent, Op: op, Err: ErrInvalidState}
	}

	coreAgent, err := c.wire.NewAgent(ctx, cc)
	if err != nil {
		log.Error(ctx, "agent registration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newAgent(coreAgent, func() {
		c.closeMu.Lock()
		defer c.closeMu.Unlock()
		if c.agent != nil && c.agent.core == coreAgent {
			c.agent = nil
		}
	})
	if pub == nil {
		log.Error(ctx, "agent wrapper unexpectedly nil", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	c.agent = pub

	log.Debug(ctx, "agent registered", "op", op)
	return pub, nil
}

// Adapter creates an Adapter wrapper for a specific iwd adapter object path.
//
// Use Daemon.Adapters to discover valid adapter paths.
func (c *Client) Adapter(ctx context.Context, path string) (*Adapter, error) {
	return clientObject(c, ctx, "Client.Adapter", path, (*connect.Wiring).NewAdapter, newAdapter)
}

// AllAdapters mints live Adapter handles for every adapter iwd currently
// exposes.
//
// It enumerates adapters via the daemon, then constructs a handle for each,
// preserving the daemon's enumeration order. Use Adapter to obtain a single
// adapter by path, or Daemon.Adapters for lightweight references without
// constructing handles.
func (c *Client) AllAdapters(ctx context.Context) ([]*Adapter, error) {
	const op = "Client.AllAdapters"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil || c.daemon == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	// Enumeration is the daemon's job; construction is the client's.
	refs, err := c.daemon.Adapters(ctx)
	if err != nil {
		log.Error(ctx, "adapter enumeration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	adapters := make([]*Adapter, 0, len(refs))
	for _, ref := range refs {
		coreAdapter, err := c.wire.NewAdapter(ctx, ref.Path)
		if err != nil {
			log.Error(ctx, "adapter wiring failed", "op", op, "path", ref.Path, "err", err)
			return nil, wrapPublicError(op, err)
		}
		pub := newAdapter(coreAdapter, ref.Path)
		if pub == nil {
			log.Error(ctx, "adapter wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		adapters = append(adapters, pub)
	}

	return adapters, nil
}

// Device creates a Device wrapper for a specific iwd device object path.
//
// Use Daemon.Devices to discover valid device paths.
func (c *Client) Device(ctx context.Context, path string) (*Device, error) {
	d, err := clientObject(c, ctx, "Client.Device", path, (*connect.Wiring).NewDevice, newDevice)
	if err != nil {
		return nil, err
	}
	return d.withResolver(c.wire.Resolver()), nil
}

// Station creates a Station wrapper for a specific iwd station object path.
//
// A station shares its object with a device (a device in station mode), so path
// is a device object path. Use Daemon.Stations to discover valid station paths.
func (c *Client) Station(ctx context.Context, path string) (*Station, error) {
	st, err := clientObject(c, ctx, "Client.Station", path, (*connect.Wiring).NewStation, wrapStation)
	if err != nil {
		return nil, err
	}
	// A station's name is its device's Name, which lives on a different interface;
	// resolve it best-effort so a single-lookup station is named like an
	// enumerated one. Failure leaves Name() == "".
	st.name = c.resolveStationName(ctx, path)
	st.resolver = c.wire.Resolver()
	st.registerSignalAgent = c.wire.NewSignalLevelAgent
	st.newSimpleConfig = c.wire.NewSimpleConfiguration
	return st, nil
}

// resolveStationName best-effort resolves a station's name (the co-located
// device's Name) via one ObjectManager enumeration, returning "" on any failure.
func (c *Client) resolveStationName(ctx context.Context, path string) string {
	if c == nil || c.daemon == nil {
		return ""
	}
	refs, err := c.daemon.Stations(ctx)
	if err != nil {
		return ""
	}
	for _, r := range refs {
		if r.Path == path {
			return r.Name
		}
	}
	return ""
}

// AllDevices mints live Device handles for every device iwd currently exposes.
//
// It enumerates devices via the daemon, then constructs a handle for each,
// preserving the daemon's enumeration order. Use Device to obtain a single
// device by path, or Daemon.Devices for lightweight references without
// constructing handles.
func (c *Client) AllDevices(ctx context.Context) ([]*Device, error) {
	const op = "Client.AllDevices"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil || c.daemon == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	// Enumeration is the daemon's job; construction is the client's.
	refs, err := c.daemon.Devices(ctx)
	if err != nil {
		log.Error(ctx, "device enumeration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	devices := make([]*Device, 0, len(refs))
	for _, ref := range refs {
		coreDevice, err := c.wire.NewDevice(ctx, ref.Path)
		if err != nil {
			log.Error(ctx, "device wiring failed", "op", op, "path", ref.Path, "err", err)
			return nil, wrapPublicError(op, err)
		}
		pub := newDevice(coreDevice, ref.Path).withResolver(c.wire.Resolver())
		if pub == nil {
			log.Error(ctx, "device wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		devices = append(devices, pub)
	}

	return devices, nil
}

// AllStations mints live Station handles for every station iwd currently
// exposes.
//
// It enumerates stations via the daemon, then constructs a handle for each,
// preserving the daemon's enumeration order. Use Station to obtain a single
// station by path, or Daemon.Stations for lightweight references without
// constructing handles.
func (c *Client) AllStations(ctx context.Context) ([]*Station, error) {
	const op = "Client.AllStations"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil || c.daemon == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	// Enumeration is the daemon's job; construction is the client's.
	refs, err := c.daemon.Stations(ctx)
	if err != nil {
		log.Error(ctx, "station enumeration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	stations := make([]*Station, 0, len(refs))
	for _, ref := range refs {
		coreStation, err := c.wire.NewStation(ctx, ref.Path)
		if err != nil {
			log.Error(ctx, "station wiring failed", "op", op, "path", ref.Path, "err", err)
			return nil, wrapPublicError(op, err)
		}
		pub := newStation(coreStation, ref.Path, ref.Name).withResolver(c.wire.Resolver()).withSignalMonitor(c.wire.NewSignalLevelAgent).withSimpleConfiguration(c.wire.NewSimpleConfiguration)
		if pub == nil {
			log.Error(ctx, "station wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		stations = append(stations, pub)
	}

	return stations, nil
}

// BasicServiceSet creates a BasicServiceSet wrapper for a specific iwd BSS
// object path.
//
// Use Daemon.BasicServiceSets to discover valid BSS paths.
func (c *Client) BasicServiceSet(ctx context.Context, path string) (*BasicServiceSet, error) {
	return clientObject(c, ctx, "Client.BasicServiceSet", path, (*connect.Wiring).NewBasicServiceSet, newBasicServiceSet)
}

// AllBasicServiceSets mints live BasicServiceSet handles for every BSS iwd
// currently exposes.
//
// It enumerates BSSes via the daemon, then constructs a handle for each,
// preserving the daemon's enumeration order. Use BasicServiceSet to obtain a
// single BSS by path, or Daemon.BasicServiceSets for lightweight references
// without constructing handles.
func (c *Client) AllBasicServiceSets(ctx context.Context) ([]*BasicServiceSet, error) {
	const op = "Client.AllBasicServiceSets"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil || c.daemon == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	// Enumeration is the daemon's job; construction is the client's.
	refs, err := c.daemon.BasicServiceSets(ctx)
	if err != nil {
		log.Error(ctx, "basic service set enumeration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	bsses := make([]*BasicServiceSet, 0, len(refs))
	for _, ref := range refs {
		coreBSS, err := c.wire.NewBasicServiceSet(ctx, ref.Path)
		if err != nil {
			log.Error(ctx, "basic service set wiring failed", "op", op, "path", ref.Path, "err", err)
			return nil, wrapPublicError(op, err)
		}
		pub := newBasicServiceSet(coreBSS, ref.Path)
		if pub == nil {
			log.Error(ctx, "basic service set wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		bsses = append(bsses, pub)
	}

	return bsses, nil
}

// Network creates a Network wrapper for a specific iwd network object path.
//
// Use Daemon.Networks to discover valid network paths.
func (c *Client) Network(ctx context.Context, path string) (*Network, error) {
	n, err := clientObject(c, ctx, "Client.Network", path, (*connect.Wiring).NewNetwork, newNetwork)
	if err != nil {
		return nil, err
	}
	return n.withResolver(c.wire.Resolver()), nil
}

// AllNetworks mints live Network handles for every network iwd currently
// exposes.
//
// It enumerates networks via the daemon, then constructs a handle for each,
// preserving the daemon's enumeration order. Use Network to obtain a single
// network by path, or Daemon.Networks for lightweight references without
// constructing handles.
func (c *Client) AllNetworks(ctx context.Context) ([]*Network, error) {
	const op = "Client.AllNetworks"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil || c.daemon == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	// Enumeration is the daemon's job; construction is the client's.
	refs, err := c.daemon.Networks(ctx)
	if err != nil {
		log.Error(ctx, "network enumeration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	networks := make([]*Network, 0, len(refs))
	for _, ref := range refs {
		coreNetwork, err := c.wire.NewNetwork(ctx, ref.Path)
		if err != nil {
			log.Error(ctx, "network wiring failed", "op", op, "path", ref.Path, "err", err)
			return nil, wrapPublicError(op, err)
		}
		pub := newNetwork(coreNetwork, ref.Path).withResolver(c.wire.Resolver())
		if pub == nil {
			log.Error(ctx, "network wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		networks = append(networks, pub)
	}

	return networks, nil
}

// KnownNetwork creates a KnownNetwork wrapper for a specific iwd known-network
// object path.
//
// Use Daemon.KnownNetworks to discover valid known-network paths.
func (c *Client) KnownNetwork(ctx context.Context, path string) (*KnownNetwork, error) {
	return clientObject(c, ctx, "Client.KnownNetwork", path, (*connect.Wiring).NewKnownNetwork, newKnownNetwork)
}

// AllKnownNetworks mints live KnownNetwork handles for every known network iwd
// currently exposes.
//
// It enumerates known networks via the daemon, then constructs a handle for each,
// preserving the daemon's enumeration order. Use KnownNetwork to obtain a single
// known network by path, or Daemon.KnownNetworks for lightweight references
// without constructing handles.
func (c *Client) AllKnownNetworks(ctx context.Context) ([]*KnownNetwork, error) {
	const op = "Client.AllKnownNetworks"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil || c.daemon == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	// Enumeration is the daemon's job; construction is the client's.
	refs, err := c.daemon.KnownNetworks(ctx)
	if err != nil {
		log.Error(ctx, "known network enumeration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	known := make([]*KnownNetwork, 0, len(refs))
	for _, ref := range refs {
		coreKnownNetwork, err := c.wire.NewKnownNetwork(ctx, ref.Path)
		if err != nil {
			log.Error(ctx, "known network wiring failed", "op", op, "path", ref.Path, "err", err)
			return nil, wrapPublicError(op, err)
		}
		pub := newKnownNetwork(coreKnownNetwork, ref.Path)
		if pub == nil {
			log.Error(ctx, "known network wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		known = append(known, pub)
	}

	return known, nil
}
