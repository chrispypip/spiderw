package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdAgentManagerIface is the fully qualified D-Bus interface name for the iwd
// agent manager. It lives on the daemon object (IwdDaemonPath).
const IwdAgentManagerIface = IwdService + ".AgentManager"

// IwdAgentIface is the fully qualified D-Bus interface name for an iwd
// credentials agent. Unlike every other iwd interface this package wraps, an
// agent is an object that *we* export and iwd calls into when it needs
// credentials during a connect.
const IwdAgentIface = IwdService + ".Agent"

// IwdAgentErrorCanceled is the D-Bus error name an agent returns to decline a
// credentials request (for example, the user dismissed a passphrase prompt).
const IwdAgentErrorCanceled = IwdAgentIface + ".Error.Canceled"

// AgentManager provides typed access to the iwd AgentManager D-Bus interface,
// used to register and unregister a credentials agent.
type AgentManager struct {
	conn  *dbus.Conn
	intro caller
}

// NewAgentManager returns an AgentManager bound to the iwd daemon object.
//
// Unlike NewDaemon (whose interface is experimental and may be absent),
// AgentManager is a standard iwd interface that is always present on a working
// daemon, so its absence is an error rather than an expected (nil, nil).
func NewAgentManager(ctx context.Context, conn *dbus.Conn) (*AgentManager, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, IwdDaemonPath)
	if err != nil {
		return nil, WrapIntrospection(string(IwdDaemonPath), err)
	}
	if !intro.HasInterface(IwdAgentManagerIface) {
		_ = intro.Close()
		// Classify an absent interface as an introspection failure so higher
		// layers treat it as "unavailable" rather than a generic failure.
		return nil, WrapIntrospection(string(IwdDaemonPath), fmt.Errorf("object %s does not implement %s", IwdDaemonPath, IwdAgentManagerIface))
	}

	return &AgentManager{conn: conn, intro: caller(intro)}, nil
}

// RegisterAgent calls AgentManager.RegisterAgent, registering the object at path
// (which the caller must have already exported via ExportAgent) as the
// credentials agent for this D-Bus connection.
func (m *AgentManager) RegisterAgent(ctx context.Context, path dbus.ObjectPath) error {
	if err := m.ensureInitialized(); err != nil {
		return WrapConnection("AgentManager.ensureInitialized", err)
	}

	if _, err := m.intro.Call(ctx, IwdAgentManagerIface, "RegisterAgent", path); err != nil {
		return wrapIwdMethod(IwdAgentManagerIface, "RegisterAgent", err)
	}
	return nil
}

// UnregisterAgent calls AgentManager.UnregisterAgent, removing a previously
// registered agent.
func (m *AgentManager) UnregisterAgent(ctx context.Context, path dbus.ObjectPath) error {
	if err := m.ensureInitialized(); err != nil {
		return WrapConnection("AgentManager.ensureInitialized", err)
	}

	if _, err := m.intro.Call(ctx, IwdAgentManagerIface, "UnregisterAgent", path); err != nil {
		return wrapIwdMethod(IwdAgentManagerIface, "UnregisterAgent", err)
	}
	return nil
}

// ensureInitialized verifies that m has been initialized by NewAgentManager.
func (m *AgentManager) ensureInitialized() error {
	if m.intro == nil {
		return ErrAgentManagerUninitialized
	}
	return nil
}

// AgentHandler holds the callbacks invoked when iwd calls into the agent object
// we export. Each request callback returns the requested credential, or an error
// to decline; a nil callback also declines. Declining maps to the
// IwdAgentErrorCanceled D-Bus error returned to iwd.
//
// network is the iwd Network object path the request concerns. Cancel and
// Release are notifications (iwd ignores their return), so they take no error.
type AgentHandler struct {
	// RequestPassphrase supplies the passphrase for a PSK network.
	RequestPassphrase func(ctx context.Context, network dbus.ObjectPath) (string, error)

	// RequestPrivateKeyPassphrase supplies the passphrase protecting an 802.1x
	// private key.
	RequestPrivateKeyPassphrase func(ctx context.Context, network dbus.ObjectPath) (string, error)

	// RequestUserNameAndPassword supplies the username and password for an
	// 802.1x network.
	RequestUserNameAndPassword func(ctx context.Context, network dbus.ObjectPath) (user, password string, err error)

	// RequestUserPassword supplies the password for an 802.1x network whose
	// username iwd already knows.
	RequestUserPassword func(ctx context.Context, network dbus.ObjectPath, user string) (string, error)

	// Cancel is called when iwd aborts a pending request. reason is one of iwd's
	// cancel reasons (for example "out-of-range", "user-canceled", "timed-out").
	Cancel func(reason string)

	// Release is called when iwd no longer needs the agent (it was unregistered
	// or replaced).
	Release func()
}

// ExportAgent exports an object implementing net.connman.iwd.Agent at path on
// conn, dispatching iwd's calls to h. The returned unexport function removes the
// object; callers should invoke it after UnregisterAgent.
func ExportAgent(conn *dbus.Conn, path dbus.ObjectPath, h AgentHandler) (unexport func() error, err error) {
	if conn == nil {
		return nil, WrapConnection("ExportAgent", ErrDBusConnection)
	}
	if !path.IsValid() {
		return nil, WrapConnection("ExportAgent", fmt.Errorf("invalid agent object path %q", path))
	}

	obj := &agentObject{handler: h}
	if err := conn.Export(obj, path, IwdAgentIface); err != nil {
		return nil, WrapConnection("ExportAgent", err)
	}

	unexport = func() error {
		return conn.Export(nil, path, IwdAgentIface)
	}
	return unexport, nil
}

// agentObject is the unexported D-Bus object that implements
// net.connman.iwd.Agent by dispatching to an AgentHandler. Its exported methods
// match iwd's Agent interface signatures.
type agentObject struct {
	handler AgentHandler
}

// canceledError builds the D-Bus error iwd expects when an agent declines.
func canceledError(err error) *dbus.Error {
	msg := "credentials request canceled"
	if err != nil {
		msg = err.Error()
	}
	return dbus.NewError(IwdAgentErrorCanceled, []interface{}{msg})
}

// dispatchCredential runs a single-secret request callback, mapping a nil
// callback or any returned error to the Canceled D-Bus error.
func dispatchCredential(fn func(ctx context.Context, network dbus.ObjectPath) (string, error), network dbus.ObjectPath) (string, *dbus.Error) {
	if fn == nil {
		return "", canceledError(nil)
	}
	secret, err := fn(context.Background(), network)
	if err != nil {
		return "", canceledError(err)
	}
	return secret, nil
}

// RequestPassphrase implements net.connman.iwd.Agent.RequestPassphrase.
func (a *agentObject) RequestPassphrase(network dbus.ObjectPath) (string, *dbus.Error) {
	return dispatchCredential(a.handler.RequestPassphrase, network)
}

// RequestPrivateKeyPassphrase implements
// net.connman.iwd.Agent.RequestPrivateKeyPassphrase.
func (a *agentObject) RequestPrivateKeyPassphrase(network dbus.ObjectPath) (string, *dbus.Error) {
	return dispatchCredential(a.handler.RequestPrivateKeyPassphrase, network)
}

// RequestUserNameAndPassword implements
// net.connman.iwd.Agent.RequestUserNameAndPassword.
func (a *agentObject) RequestUserNameAndPassword(network dbus.ObjectPath) (string, string, *dbus.Error) {
	if a.handler.RequestUserNameAndPassword == nil {
		return "", "", canceledError(nil)
	}
	user, password, err := a.handler.RequestUserNameAndPassword(context.Background(), network)
	if err != nil {
		return "", "", canceledError(err)
	}
	return user, password, nil
}

// RequestUserPassword implements net.connman.iwd.Agent.RequestUserPassword.
func (a *agentObject) RequestUserPassword(network dbus.ObjectPath, user string) (string, *dbus.Error) {
	if a.handler.RequestUserPassword == nil {
		return "", canceledError(nil)
	}
	password, err := a.handler.RequestUserPassword(context.Background(), network, user)
	if err != nil {
		return "", canceledError(err)
	}
	return password, nil
}

// Cancel implements net.connman.iwd.Agent.Cancel.
func (a *agentObject) Cancel(reason string) *dbus.Error {
	if a.handler.Cancel != nil {
		a.handler.Cancel(reason)
	}
	return nil
}

// Release implements net.connman.iwd.Agent.Release.
func (a *agentObject) Release() *dbus.Error {
	if a.handler.Release != nil {
		a.handler.Release()
	}
	return nil
}
