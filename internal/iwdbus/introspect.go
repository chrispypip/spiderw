package iwdbus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

// DBusIntrospectableIface and related constants name standard D-Bus interfaces
// and methods used by spiderw.
const (
	// DBusIntrospectableIface is the standard D-Bus introspection interface.
	DBusIntrospectableIface = "org.freedesktop.DBus.Introspectable"

	// DBusIntrospectFn is the fully qualified D-Bus introspection method.
	DBusIntrospectFn = DBusIntrospectableIface + ".Introspect"

	// DBusObjectManagerIface is the standard D-Bus ObjectManager interface.
	DBusObjectManagerIface = "org.freedesktop.DBus.ObjectManager"

	// DBusObjectManagerGetManagedObjects is the fully qualified
	// ObjectManager.GetManagedObjects method.
	DBusObjectManagerGetManagedObjects = DBusObjectManagerIface + ".GetManagedObjects"
)

const (
	socketSettleDelay  = 10 * time.Millisecond
	dbusSignalCapacity = 64
)

type signalHandler struct {
	id uint64
	fn func(*dbus.Signal)
}

func getManagedObjects(ctx context.Context, conn *dbus.Conn, service string) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil dbus conn")
	}

	obj := conn.Object(service, dbus.ObjectPath("/"))
	call := obj.CallWithContext(ctx, DBusObjectManagerGetManagedObjects, 0)
	if call.Err != nil {
		return nil, call.Err
	}

	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{}
	if err := call.Store(&objects); err != nil {
		return nil, err
	}
	return objects, nil
}

// IntrospectedObject represents a D-Bus object with runtime introspected
// interfaces.
//
// -----------------------------------------------------------------------------
// Concurrency & Lifecycle Contract
// -----------------------------------------------------------------------------
//
// IntrospectedObject manages three concurrent concerns:
//
//   1. A single dispatcher goroutine (dispatchLoop)
//   2. Zero or more handler goroutines spawned per received signal
//   3. A Close() path that must safely shut down both without races or panics
//
// The following invariants MUST always hold:
//
//   * dispatchWG tracks the lifetime of the dispatcher goroutine.
//     - startDispatcher() increments dispatchWG exactly once.
//     - dispatchLoop() decrements dispatchWG exactly once on exit.
//
//   * wg tracks the lifetime of all handler goroutines.
//     - wg.Add(n) MUST NOT race with wg.Wait().
//     - Every handler goroutine MUST call wg.Done() exactly once.
//
//   * handlerMu serializes handler lifecycle transitions.
//     - handlerMu protects wg.Add(...) and the transition to shutdown.
//     - Close() sets closing=true while holding handlerMu, then waits on wg.
//     - dispatchLoop() checks closing under handlerMu before calling wg.Add(...).
//
//   * Once Close() begins (closing=true):
//     - No new handler goroutines may be spawned.
//     - dispatchLoop will exit promptly via context cancellation.
//     - Close() may abandon the dispatcher after a timeout, but handler
//       spawning remains permanently disabled.
//
//   * Close() is idempotent and safe to call concurrently.
//
// This design ensures:
//   - Slow or misbehaving handlers cannot block the dispatcher.
//   - Handler panics are isolated.
//   - Close() cannot panic due to WaitGroup misuse (Add concurrent with Wait).
//   - Signals emitted concurrently with Close() are either handled fully
//     or safely dropped once shutdown begins.
// -----------------------------------------------------------------------------

// IntrospectedObject wraps a D-Bus object with introspection and signal-dispatch helpers.
type IntrospectedObject struct {
	Conn    *dbus.Conn
	BusName string
	Path    dbus.ObjectPath

	ctx    context.Context
	cancel context.CancelFunc
	ifaces map[string]*introspect.Interface

	mu         sync.RWMutex
	wg         sync.WaitGroup
	dispatchWG sync.WaitGroup
	handlerMu  sync.Mutex
	startOnce  sync.Once
	sigCh      chan *dbus.Signal

	handlersExact    map[string][]signalHandler // "iface.member"
	handlersWildcard map[string][]signalHandler // "*.member", "iface.*", or "*.*"
	nextHandlerID    atomic.Uint64

	closing bool
	once    sync.Once
}

// NewIntrospectedObject introspects the object at path on busName.
func NewIntrospectedObject(ctx context.Context, conn *dbus.Conn, busName string, path dbus.ObjectPath) (*IntrospectedObject, error) {
	obj := conn.Object(busName, path)
	node, err := introspect.Call(obj)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}

	cctx, cancel := context.WithCancel(ctx)
	intro := &IntrospectedObject{
		Conn:             conn,
		BusName:          busName,
		Path:             path,
		ctx:              cctx,
		cancel:           cancel,
		ifaces:           map[string]*introspect.Interface{},
		sigCh:            make(chan *dbus.Signal, dbusSignalCapacity),
		handlersExact:    map[string][]signalHandler{},
		handlersWildcard: map[string][]signalHandler{},
	}

	for i := range node.Interfaces {
		intro.ifaces[node.Interfaces[i].Name] = &node.Interfaces[i]
	}

	// Start listening to signals.
	conn.Signal(intro.sigCh)

	return intro, nil
}

// Close unregisters signal handling for the object and waits briefly for in-flight
// dispatcher work to stop.
//
// Close does not close the underlying D-Bus connection.
func (i *IntrospectedObject) Close() error {
	if i == nil {
		return nil
	}

	i.once.Do(func() {
		if i.cancel != nil {
			i.cancel()
		}
		if i.Conn != nil && i.sigCh != nil {
			i.Conn.RemoveSignal(i.sigCh)
		}

		_ = waitWithTimeout(&i.dispatchWG, 100*time.Millisecond)

		i.handlerMu.Lock()
		// Prevent any new handler goroutines and serialize wg.Wait with wg.Add.
		i.closing = true
		_ = waitWithTimeout(&i.wg, 100*time.Millisecond)
		i.handlerMu.Unlock()

		// Give OS time to settle the socket.
		time.Sleep(socketSettleDelay)
	})

	return nil
}

// Call calls iface.method with args and returns raw body.
func (i *IntrospectedObject) Call(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
	fullMethod := iface + "." + method

	obj := i.Conn.Object(i.BusName, i.Path)
	call := obj.CallWithContext(ctx, fullMethod, 0, args...)
	if call.Err != nil {
		return nil, WrapMethod(iface, method, call.Err)
	}

	return call.Body, nil
}

// GetProperty fetches a property via org.freedesktop.DBus.Properties.Get.
func (i *IntrospectedObject) GetProperty(ctx context.Context, iface, prop string) (interface{}, error) {
	obj := i.Conn.Object(i.BusName, i.Path)
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, iface, prop).Store(&v); err != nil {
		// Return the raw D-Bus error; the typed caller (e.g. Adapter.GetModel)
		// owns property-error wrapping so it is applied exactly once.
		return nil, err
	}

	return v.Value(), nil
}

// GetAll fetches every property of iface in a single
// org.freedesktop.DBus.Properties.GetAll call.
func (i *IntrospectedObject) GetAll(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
	obj := i.Conn.Object(i.BusName, i.Path)
	var props map[string]dbus.Variant
	if err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, iface).Store(&props); err != nil {
		// Return the raw D-Bus error; the typed caller owns wrapping so it is
		// applied exactly once.
		return nil, err
	}

	return props, nil
}

// SetProperty uses Properties.Set.
func (i *IntrospectedObject) SetProperty(ctx context.Context, iface, prop string, val interface{}) error {
	obj := i.Conn.Object(i.BusName, i.Path)
	call := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Set", 0, iface, prop, dbus.MakeVariant(val))
	if call.Err != nil {
		// Return the raw D-Bus error; the typed caller owns property-error
		// wrapping so it is applied exactly once.
		return call.Err
	}

	return nil
}

// HasInterface checks if introspected interface exists.
func (i *IntrospectedObject) HasInterface(name string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.ifaces[name]
	return ok
}

// UnsubscribeFunc removes a previously registered signal handler.
type UnsubscribeFunc func() error

// Unsubscribe calls u. A nil UnsubscribeFunc is a no-op.
func (u UnsubscribeFunc) Unsubscribe() error {
	if u == nil {
		return nil
	}
	return u()
}

// RegisterSignalHandlerWithUnsubscribe registers a handler and returns a
// function that unregisters the local handlers.
//
// Unsubscribe is idempotent. A signal that has already been resolved by the
// dispatcher may still invoke the callback once after unsubscribe returns, but
// future dispatches will not include the removed handler.
func (i *IntrospectedObject) RegisterSignalHandlerWithUnsubscribe(iface, member string, handler func(*dbus.Signal)) (UnsubscribeFunc, error) {
	if handler == nil {
		return nil, fmt.Errorf("RegisterSignalHandler: handler cannot be nil")
	}
	if member == "" {
		return nil, fmt.Errorf("RegisterSignalHandler: member cannot be empty")
	}

	i.startDispatcher()

	var (
		matchIface  string
		matchMember string
		rule        string
	)

	i.mu.Lock()

	// Ensure maps exist
	if i.handlersExact == nil {
		i.handlersExact = make(map[string][]signalHandler)
	}
	if i.handlersWildcard == nil {
		i.handlersWildcard = make(map[string][]signalHandler)
	}

	id := i.nextHandlerID.Add(1)
	stored := signalHandler{id: id, fn: handler}
	var bucket *map[string][]signalHandler
	var handlerKey string

	// Store handler
	switch {
	case iface == "*" && member == "*":
		handlerKey = "*"
		i.handlersWildcard[handlerKey] = append(i.handlersWildcard[handlerKey], stored)
		bucket = &i.handlersWildcard

	case iface == "*":
		handlerKey = member
		i.handlersWildcard[handlerKey] = append(i.handlersWildcard[handlerKey], stored)
		bucket = &i.handlersWildcard
		matchMember = member

	case member == "*":
		handlerKey = iface + ".*"
		i.handlersWildcard[handlerKey] = append(i.handlersWildcard[handlerKey], stored)
		bucket = &i.handlersWildcard
		matchIface = iface

	default:
		handlerKey = iface + "." + member
		i.handlersExact[handlerKey] = append(i.handlersExact[handlerKey], stored)
		bucket = &i.handlersExact
		matchIface = iface
		matchMember = member
	}

	var once sync.Once
	unsubscribe := UnsubscribeFunc(func() error {
		once.Do(func() {
			i.mu.Lock()
			defer i.mu.Unlock()

			handlers := (*bucket)[handlerKey]
			for idx, h := range handlers {
				if h.id == id {
					copy(handlers[idx:], handlers[idx+1:])
					handlers[len(handlers)-1] = signalHandler{}
					handlers = handlers[:len(handlers)-1]
					break
				}
			}
			if len(handlers) == 0 {
				delete(*bucket, handlerKey)
				return
			}
			(*bucket)[handlerKey] = handlers
		})
		return nil
	})

	i.mu.Unlock()

	// No live D-Bus connection: store handlers only.
	if i.Conn == nil {
		return unsubscribe, nil
	}

	// Build D-Bus match rule.
	rule = fmt.Sprintf("type='signal',path='%s'", i.Path)

	if matchIface != "" {
		rule += fmt.Sprintf(",interface='%s'", matchIface)
	}
	if matchMember != "" {
		rule += fmt.Sprintf(",member='%s'", matchMember)
	}

	if err := i.Conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
		_ = unsubscribe()
		return nil, WrapMethod("org.freedesktop.DBus", "AddMatch", err)
	}

	return unsubscribe, nil
}

// RegisterSignalHandler registers a handler for iface.member signals
// by adding a match rule.
func (i *IntrospectedObject) RegisterSignalHandler(iface, member string, handler func(*dbus.Signal)) error {
	_, err := i.RegisterSignalHandlerWithUnsubscribe(iface, member, handler)
	return err
}

func (i *IntrospectedObject) startDispatcher() {
	i.startOnce.Do(func() {
		i.dispatchWG.Go(func() {
			i.dispatchLoop()
		})
	})
}

func (i *IntrospectedObject) dispatchLoop() {
	for {
		select {
		case <-i.ctx.Done():
			return
		case sig, ok := <-i.sigCh:
			if !ok {
				return
			}
			if sig == nil || sig.Name == "" {
				continue
			}

			hs := i.resolveHandlers(sig)
			i.handlerMu.Lock()
			if i.closing {
				i.handlerMu.Unlock()
				return
			}
			// Serialize handler spawning with Close().
			// Once closing=true, no new handler goroutines may be added to wg.
			i.wg.Add(len(hs))
			i.handlerMu.Unlock()

			for _, h := range hs {
				go func(hh func(*dbus.Signal)) {
					defer i.wg.Done()
					safeInvoke(hh, sig)
				}(h)
			}
		}
	}
}

func (i *IntrospectedObject) resolveHandlers(sig *dbus.Signal) []func(*dbus.Signal) {
	iface, member := splitSignalName(sig.Name)
	key := iface + "." + member

	i.mu.RLock()
	defer i.mu.RUnlock()

	var out []func(*dbus.Signal)
	for _, h := range i.handlersExact[key] {
		out = append(out, h.fn)
	}
	for _, h := range i.handlersWildcard[member] {
		out = append(out, h.fn)
	}
	for _, h := range i.handlersWildcard[iface+".*"] {
		out = append(out, h.fn)
	}
	for _, h := range i.handlersWildcard["*"] {
		out = append(out, h.fn)
	}
	return out
}

func safeInvoke(fn func(*dbus.Signal), sig *dbus.Signal) {
	defer func() { _ = recover() }()
	fn(sig)
}

func waitWithTimeout(wg *sync.WaitGroup, d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}
