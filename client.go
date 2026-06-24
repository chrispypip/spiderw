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
		defer c.closeMu.Unlock()
		c.closed = true
		c.daemon = nil
		if c.cleanup == nil {
			return
		}

		err := c.cleanup()
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

// Adapter creates an Adapter wrapper for a specific iwd adapter object path.
//
// Use Daemon.Adapters to discover valid adapter paths.
func (c *Client) Adapter(ctx context.Context, path string) (*Adapter, error) {
	const op = "Client.Adapter"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op, "path", path)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreAdapter, err := c.wire.NewAdapter(ctx, path)
	if err != nil {
		log.Error(ctx, "adapter wiring failed", "op", op, "path", path, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newAdapter(coreAdapter, path)
	if pub == nil {
		log.Error(ctx, "adapter wrapper unexpectedly nil", "op", op, "path", path)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return pub, nil
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
	const op = "Client.Device"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op, "path", path)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreDevice, err := c.wire.NewDevice(ctx, path)
	if err != nil {
		log.Error(ctx, "device wiring failed", "op", op, "path", path, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newDevice(coreDevice, path)
	if pub == nil {
		log.Error(ctx, "device wrapper unexpectedly nil", "op", op, "path", path)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return pub, nil
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
		pub := newDevice(coreDevice, ref.Path)
		if pub == nil {
			log.Error(ctx, "device wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		devices = append(devices, pub)
	}

	return devices, nil
}

// BasicServiceSet creates a BasicServiceSet wrapper for a specific iwd BSS
// object path.
//
// Use Daemon.BasicServiceSets to discover valid BSS paths.
func (c *Client) BasicServiceSet(ctx context.Context, path string) (*BasicServiceSet, error) {
	const op = "Client.BasicServiceSet"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op, "path", path)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreBSS, err := c.wire.NewBasicServiceSet(ctx, path)
	if err != nil {
		log.Error(ctx, "basic service set wiring failed", "op", op, "path", path, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newBasicServiceSet(coreBSS, path)
	if pub == nil {
		log.Error(ctx, "basic service set wrapper unexpectedly nil", "op", op, "path", path)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return pub, nil
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
	const op = "Client.Network"
	log := logging.FromContext(ctx)

	if c == nil {
		log.Error(ctx, "client uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	if c.closed {
		log.Error(ctx, "client already closed", "op", op, "path", path)
		return nil, &Error{Kind: KindInvalidState, Resource: ResourceClient, Op: op, Err: ErrInvalidState}
	}
	if c.wire == nil {
		log.Error(ctx, "client wiring uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreNetwork, err := c.wire.NewNetwork(ctx, path)
	if err != nil {
		log.Error(ctx, "network wiring failed", "op", op, "path", path, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newNetwork(coreNetwork, path)
	if pub == nil {
		log.Error(ctx, "network wrapper unexpectedly nil", "op", op, "path", path)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return pub, nil
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
		pub := newNetwork(coreNetwork, ref.Path)
		if pub == nil {
			log.Error(ctx, "network wrapper unexpectedly nil", "op", op, "path", ref.Path)
			return nil, wrapPublicError(op, ErrInternal)
		}
		networks = append(networks, pub)
	}

	return networks, nil
}
