package spiderw

import (
	"context"
	"sync"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// AgentConfig supplies the callbacks iwd invokes when it needs credentials to
// connect to a secured network that is not already known. Register it with
// Client.RegisterAgent.
//
// Each request callback returns the requested credential, or a non-nil error to
// decline; a nil callback also declines (iwd receives a Canceled reply). At least
// one request callback (Passphrase, PrivateKeyPassphrase, UserNameAndPassword, or
// UserPassword) must be set, or RegisterAgent returns an invalid-argument error.
//
// networkPath is the iwd Network object path the request concerns; resolve it to
// a handle with Client.Network if you need network details. The supplied context
// is canceled if the request is aborted (for example by iwd's Cancel or by
// Agent.Unregister), so long-running callbacks such as interactive prompts should
// honor it. OnCancel and OnRelease are optional notifications.
//
// Testing status: the Passphrase (PSK) path is tested end to end against the iwd
// mock. The 802.1x callbacks (PrivateKeyPassphrase, UserNameAndPassword,
// UserPassword) are wired through every layer but are not yet exercised against
// the mock or validated on hardware; treat them as experimental.
type AgentConfig struct {
	// Passphrase supplies the passphrase for a PSK network.
	Passphrase func(ctx context.Context, networkPath string) (string, error)

	// PrivateKeyPassphrase supplies the passphrase protecting an 802.1x private
	// key.
	PrivateKeyPassphrase func(ctx context.Context, networkPath string) (string, error)

	// UserNameAndPassword supplies the username and password for an 802.1x
	// network.
	UserNameAndPassword func(ctx context.Context, networkPath string) (user, password string, err error)

	// UserPassword supplies the password for an 802.1x network whose username iwd
	// already knows.
	UserPassword func(ctx context.Context, networkPath, user string) (string, error)

	// OnCancel is called when iwd aborts a pending request. reason is one of
	// iwd's cancel reasons (for example "out-of-range", "user-canceled",
	// "timed-out").
	OnCancel func(reason string)

	// OnRelease is called when iwd no longer needs the agent (it was unregistered
	// or replaced).
	OnRelease func()
}

func (cfg AgentConfig) toCore() core.CredentialCallbacks {
	return core.CredentialCallbacks{
		Passphrase:           cfg.Passphrase,
		PrivateKeyPassphrase: cfg.PrivateKeyPassphrase,
		UserNameAndPassword:  cfg.UserNameAndPassword,
		UserPassword:         cfg.UserPassword,
		OnCancel:             cfg.OnCancel,
		OnRelease:            cfg.OnRelease,
	}
}

// Agent is a registered credentials agent handle returned by
// Client.RegisterAgent. Call Unregister when the agent is no longer needed; the
// owning Client also unregisters it on Close.
type Agent struct {
	core core.AgentIface

	clearOnce sync.Once
	// clear releases the owning client's agent slot so a new agent can be
	// registered after this one is unregistered.
	clear func()
}

func newAgent(c core.AgentIface, clear func()) *Agent {
	if c == nil {
		return nil
	}
	return &Agent{core: c, clear: clear}
}

// Unregister unregisters the agent from iwd and releases its resources. It is
// idempotent and safe to call even after the owning Client has been closed.
func (a *Agent) Unregister(ctx context.Context) error {
	const op = "Agent.Unregister"
	log := logging.FromContext(ctx)

	if a == nil || a.core == nil {
		log.Error(ctx, "agent uninitialized", "op", op)
		return &Error{Kind: KindInvalidState, Resource: ResourceAgent, Op: op, Err: ErrInvalidState}
	}

	err := a.core.Unregister(ctx)
	a.clearOnce.Do(func() {
		if a.clear != nil {
			a.clear()
		}
	})
	if err != nil {
		log.Error(ctx, "agent unregister failed", "op", op, "err", err)
		return wrapPublicError(op, err)
	}
	return nil
}
