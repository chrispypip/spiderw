package core

import (
	"context"
	"errors"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// CredentialCallbacks holds the normalized credential-supply callbacks driven by
// iwd's calls into a registered agent. A nil callback declines that request type
// (iwd receives a Canceled error). networkPath is the iwd Network object path the
// request concerns; resolve it to a handle at the public layer if needed.
//
// At least one request callback (Passphrase, PrivateKeyPassphrase,
// UserNameAndPassword, or UserPassword) must be set; see Validate. OnCancel and
// OnRelease are optional notifications.
type CredentialCallbacks struct {
	Passphrase           func(ctx context.Context, networkPath string) (string, error)
	PrivateKeyPassphrase func(ctx context.Context, networkPath string) (string, error)
	UserNameAndPassword  func(ctx context.Context, networkPath string) (user, password string, err error)
	UserPassword         func(ctx context.Context, networkPath, user string) (string, error)
	OnCancel             func(reason string)
	OnRelease            func()
}

// hasRequestCallback reports whether at least one credential-request callback is
// set.
func (c CredentialCallbacks) hasRequestCallback() bool {
	return c.Passphrase != nil ||
		c.PrivateKeyPassphrase != nil ||
		c.UserNameAndPassword != nil ||
		c.UserPassword != nil
}

// Validate ensures at least one credential-request callback is set, turning the
// otherwise-silent "agent that declines everything" case into an explicit error.
func (c CredentialCallbacks) Validate(op string) error {
	if !c.hasRequestCallback() {
		return WrapInvalidArgument(ResourceAgent, op, "at least one credential-request callback must be set", ErrCore)
	}
	return nil
}

// agentManagerRaw is the raw AgentManager backend used by Agent. It mirrors
// iwdbus.AgentManager so the concrete type satisfies it directly.
type agentManagerRaw interface {
	RegisterAgent(ctx context.Context, path dbus.ObjectPath) error
	UnregisterAgent(ctx context.Context, path dbus.ObjectPath) error
}

// AgentIface defines the core agent operations used by the public layer.
type AgentIface interface {
	Unregister(ctx context.Context) error
}

// Agent is the core-layer handle for a registered credentials agent. It owns the
// per-request cancellation state that iwd's Cancel calls act on, and the cleanup
// (UnregisterAgent + unexport) run by Unregister.
//
// iwd issues credential requests serially per agent, so a single in-flight
// cancel func suffices.
type Agent struct {
	callbacks CredentialCallbacks

	mu            sync.Mutex
	currentCancel context.CancelFunc
	registered    bool
	manager       agentManagerRaw
	path          dbus.ObjectPath
	unexport      func() error
}

// NewAgent creates an unregistered core Agent from callbacks and returns it
// along with the iwdbus.AgentHandler bound to its cancellation state. The caller
// (connect layer) exports the handler, registers it, then calls Bind to attach
// the manager and cleanup used by Unregister.
func NewAgent(callbacks CredentialCallbacks) (*Agent, iwdbus.AgentHandler) {
	a := &Agent{callbacks: callbacks}
	return a, a.buildHandler()
}

// Bind attaches the registered agent manager, object path, and unexport cleanup
// so Unregister can tear the agent down. The connect layer calls Bind after a
// successful RegisterAgent.
func (a *Agent) Bind(manager agentManagerRaw, path dbus.ObjectPath, unexport func() error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.manager = manager
	a.path = path
	a.unexport = unexport
	a.registered = true
}

// Unregister unregisters the agent from iwd and removes the exported object,
// canceling any in-flight credential request first. It is idempotent.
func (a *Agent) Unregister(ctx context.Context) error {
	const op = "Agent.Unregister"

	if a == nil {
		return WrapInvalidState(ResourceAgent, op, "agent wrapper was nil", ErrAgentNotInitialized)
	}

	a.mu.Lock()
	if !a.registered {
		a.mu.Unlock()
		return nil
	}
	a.registered = false
	manager := a.manager
	path := a.path
	unexport := a.unexport
	cancel := a.currentCancel
	a.mu.Unlock()

	// Abort a pending request so a blocked callback (for example an interactive
	// prompt) unblocks.
	if cancel != nil {
		cancel()
	}

	var unregErr error
	if manager != nil {
		if err := manager.UnregisterAgent(ctx, path); err != nil {
			unregErr = WrapAgentUnavailable(op, "failed unregistering iwd agent", err)
		}
	}

	var unexportErr error
	if unexport != nil {
		if err := unexport(); err != nil {
			unexportErr = WrapAgentUnavailable(op, "failed unexporting agent object", err)
		}
	}

	return errors.Join(unregErr, unexportErr)
}

// buildHandler returns the iwdbus.AgentHandler whose request closures derive a
// cancellable context (so Cancel can abort the in-flight request) and forward to
// the normalized callbacks. A nil callback yields a nil handler func, which the
// iwdbus layer maps to a Canceled reply.
func (a *Agent) buildHandler() iwdbus.AgentHandler {
	return iwdbus.AgentHandler{
		RequestPassphrase:           a.wrapSecret(a.callbacks.Passphrase),
		RequestPrivateKeyPassphrase: a.wrapSecret(a.callbacks.PrivateKeyPassphrase),
		RequestUserNameAndPassword:  a.wrapUserNameAndPassword(),
		RequestUserPassword:         a.wrapUserPassword(),
		Cancel:                      a.handleCancel,
		Release:                     a.handleRelease,
	}
}

// beginRequest derives a cancellable context for one in-flight request and records
// its cancel so Cancel/Unregister can abort it. The returned done func clears the
// record and releases the context.
func (a *Agent) beginRequest() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.currentCancel = cancel
	a.mu.Unlock()
	return ctx, func() {
		a.mu.Lock()
		a.currentCancel = nil
		a.mu.Unlock()
		cancel()
	}
}

func (a *Agent) wrapSecret(fn func(ctx context.Context, networkPath string) (string, error)) func(ctx context.Context, network dbus.ObjectPath) (string, error) {
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, network dbus.ObjectPath) (string, error) {
		// The dispatch ctx is intentionally not propagated; beginRequest derives a
		// fresh cancellable context so Cancel/Unregister can abort the request.
		reqCtx, done := a.beginRequest()
		defer done()
		return fn(reqCtx, string(network))
	}
}

func (a *Agent) wrapUserNameAndPassword() func(ctx context.Context, network dbus.ObjectPath) (string, string, error) {
	fn := a.callbacks.UserNameAndPassword
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, network dbus.ObjectPath) (string, string, error) {
		// The dispatch ctx is intentionally not propagated; beginRequest derives a
		// fresh cancellable context so Cancel/Unregister can abort the request.
		reqCtx, done := a.beginRequest()
		defer done()
		return fn(reqCtx, string(network))
	}
}

func (a *Agent) wrapUserPassword() func(ctx context.Context, network dbus.ObjectPath, user string) (string, error) {
	fn := a.callbacks.UserPassword
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, network dbus.ObjectPath, user string) (string, error) {
		// The dispatch ctx is intentionally not propagated; beginRequest derives a
		// fresh cancellable context so Cancel/Unregister can abort the request.
		reqCtx, done := a.beginRequest()
		defer done()
		return fn(reqCtx, string(network), user)
	}
}

func (a *Agent) handleCancel(reason string) {
	a.mu.Lock()
	cancel := a.currentCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if a.callbacks.OnCancel != nil {
		a.callbacks.OnCancel(reason)
	}
}

func (a *Agent) handleRelease() {
	if a.callbacks.OnRelease != nil {
		a.callbacks.OnRelease()
	}
}
