package mock

import (
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// securedNetworkPassphrase is the passphrase the mock agent must supply for the
// secured, not-yet-known network to connect. Integration tests use this literal.
const securedNetworkPassphrase = "mock-secret-passphrase"

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
// when the path was never registered.
func (r *agentRegistry) unregister(sender dbus.Sender, path dbus.ObjectPath) *dbus.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.set || r.path != path {
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
