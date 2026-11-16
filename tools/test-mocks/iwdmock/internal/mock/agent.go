package mock

import (
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// securedNetworkPassphrase is the passphrase the mock agent must supply for the
// secured, not-yet-known network to connect. Integration tests use this literal.
const securedNetworkPassphrase = "mock-secret-passphrase"

// The 802.1x (EAP) networks' expected credentials, one set per method the mock
// drives. The mock agent must supply these for the matching network to connect;
// integration tests use these literals.
const (
	// username+password (RequestUserNameAndPassword) - PeapNet.
	enterpriseNetworkUser     = "mock-eap-user"
	enterpriseNetworkPassword = "mock-eap-password"
	// password-only (RequestUserPassword) - TtlsNet. The mock supplies this
	// identity (as iwd would from the network config) and expects the password.
	enterprisePasswordIdentity = "mock-eap-identity"
	enterprisePasswordOnly     = "mock-ttls-password"
	// private-key passphrase (RequestPrivateKeyPassphrase) - TlsNet (EAP-TLS).
	enterpriseKeyPassphrase = "mock-tls-key-pass"
)

// agentRegistry tracks the single credentials agent registered via
// AgentManager, and calls back into it (over D-Bus) when the mock needs a
// passphrase. It mirrors iwd's one-agent-per-connection model.
type agentRegistry struct {
	mu     sync.Mutex
	conn   *dbus.Conn
	sender string
	path   dbus.ObjectPath
	set    bool
}

var agents agentRegistry

// bindConn records the connection used to call back into a registered agent.
func (r *agentRegistry) bindConn(conn *dbus.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conn = conn
}

// register stores the agent for sender at path, mirroring iwd's AlreadyExists
// rejection of a second agent.
func (r *agentRegistry) register(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.set {
		return dbus.NewError(iwdbus.IwdErrorAlreadyExists, []interface{}{"an agent is already registered"})
	}
	r.sender = string(sender)
	r.path = path
	r.set = true
	return nil
}

// unregister clears the registered agent, mirroring iwd's NotFound rejection
// when the path was never registered, or by a different client (sender).
func (r *agentRegistry) unregister(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.set || r.path != path || r.sender != string(sender) {
		return dbus.NewError(iwdbus.IwdErrorNotFound, []interface{}{"no such agent registered"})
	}
	r.set = false
	r.sender = ""
	r.path = ""
	return nil
}

// requestPassphrase calls the registered agent's RequestPassphrase for network.
// ok is false when no agent is registered (the caller should reply NoAgent).
func (r *agentRegistry) requestPassphrase(network dbus.ObjectPath) (passphrase string, ok bool, err error) {
	r.mu.Lock()
	conn, sender, path, set := r.conn, r.sender, r.path, r.set
	r.mu.Unlock()

	if !set || conn == nil {
		return "", false, nil
	}

	call := conn.Object(sender, path).Call(iwdbus.IwdAgentIface+".RequestPassphrase", 0, network)
	if call.Err != nil {
		return "", true, call.Err
	}
	if err := call.Store(&passphrase); err != nil {
		return "", true, err
	}
	return passphrase, true, nil
}

// requestUserNameAndPassword calls the registered agent's
// RequestUserNameAndPassword for an 802.1x network. ok is false when no agent is
// registered (the caller should reply NoAgent).
func (r *agentRegistry) requestUserNameAndPassword(network dbus.ObjectPath) (user, password string, ok bool, err error) {
	r.mu.Lock()
	conn, sender, path, set := r.conn, r.sender, r.path, r.set
	r.mu.Unlock()

	if !set || conn == nil {
		return "", "", false, nil
	}

	call := conn.Object(sender, path).Call(iwdbus.IwdAgentIface+".RequestUserNameAndPassword", 0, network)
	if call.Err != nil {
		return "", "", true, call.Err
	}
	if err := call.Store(&user, &password); err != nil {
		return "", "", true, err
	}
	return user, password, true, nil
}

// requestUserPassword calls the registered agent's RequestUserPassword for an
// 802.1x network whose identity the network already carries (passed as user).
// ok is false when no agent is registered.
func (r *agentRegistry) requestUserPassword(network dbus.ObjectPath, user string) (password string, ok bool, err error) {
	r.mu.Lock()
	conn, sender, path, set := r.conn, r.sender, r.path, r.set
	r.mu.Unlock()

	if !set || conn == nil {
		return "", false, nil
	}

	call := conn.Object(sender, path).Call(iwdbus.IwdAgentIface+".RequestUserPassword", 0, network, user)
	if call.Err != nil {
		return "", true, call.Err
	}
	if err := call.Store(&password); err != nil {
		return "", true, err
	}
	return password, true, nil
}

// requestPrivateKeyPassphrase calls the registered agent's
// RequestPrivateKeyPassphrase for an 802.1x (EAP-TLS) network whose client
// private key is encrypted. ok is false when no agent is registered.
func (r *agentRegistry) requestPrivateKeyPassphrase(network dbus.ObjectPath) (passphrase string, ok bool, err error) {
	r.mu.Lock()
	conn, sender, path, set := r.conn, r.sender, r.path, r.set
	r.mu.Unlock()

	if !set || conn == nil {
		return "", false, nil
	}

	call := conn.Object(sender, path).Call(iwdbus.IwdAgentIface+".RequestPrivateKeyPassphrase", 0, network)
	if call.Err != nil {
		return "", true, call.Err
	}
	if err := call.Store(&passphrase); err != nil {
		return "", true, err
	}
	return passphrase, true, nil
}

// RegisterAgent implements net.connman.iwd.AgentManager.RegisterAgent. godbus
// injects sender; the wire argument is just the object path.
func (d *Daemon) RegisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	return agents.register(sender, path)
}

// UnregisterAgent implements net.connman.iwd.AgentManager.UnregisterAgent.
func (d *Daemon) UnregisterAgent(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	return agents.unregister(sender, path)
}

// agentManagerExported reports whether the AgentManager interface should be
// exported and advertised in introspection.
func agentManagerExported() bool {
	return !*omitAgentFlag
}

// invalidPassphraseError builds the error the mock returns when an agent
// supplies the wrong passphrase, mirroring an iwd association failure.
func invalidPassphraseError() *dbus.Error {
	return dbus.NewError(iwdbus.IwdErrorFailed, []interface{}{"invalid passphrase for secured network"})
}
