package iwdbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdSignalLevelAgentIface is the fully qualified D-Bus interface name for an
// iwd signal-level agent. Like the credentials Agent, this is an object that
// *we* export and iwd calls into - here to report when the connected network's
// signal strength crosses a threshold. Unlike the credentials Agent it is
// registered per-station (Station.RegisterSignalLevelAgent), not through the
// AgentManager.
const IwdSignalLevelAgentIface = IwdService + ".SignalLevelAgent"

// SignalLevelAgentHandler holds the callbacks iwd invokes on the signal-level
// agent object we export. Both are notifications (iwd ignores their return), so
// neither returns an error and a nil callback is simply a no-op.
type SignalLevelAgentHandler struct {
	// Changed is called when the connected network's signal strength crosses one
	// of the thresholds the agent was registered with. device is the iwd device
	// object path the report concerns; level is the index into that threshold
	// list - 0 is the strongest band (stronger than the first threshold) and
	// higher values are progressively weaker.
	Changed func(device dbus.ObjectPath, level uint8)

	// Release is called when iwd no longer needs the agent (it was unregistered
	// or replaced).
	Release func()
}

// ExportSignalLevelAgent exports an object implementing
// net.connman.iwd.SignalLevelAgent at path on conn, dispatching iwd's calls to
// h. The returned unexport function removes the object; callers should invoke it
// after Station.UnregisterSignalLevelAgent.
func ExportSignalLevelAgent(conn *dbus.Conn, path dbus.ObjectPath, h SignalLevelAgentHandler) (unexport func() error, err error) {
	if conn == nil {
		return nil, WrapConnection("ExportSignalLevelAgent", ErrDBusConnection)
	}
	if !path.IsValid() {
		return nil, WrapConnection("ExportSignalLevelAgent", fmt.Errorf("invalid signal level agent object path %q", path))
	}

	obj := &signalLevelAgentObject{handler: h}
	if err := conn.Export(obj, path, IwdSignalLevelAgentIface); err != nil {
		return nil, WrapConnection("ExportSignalLevelAgent", err)
	}

	unexport = func() error {
		return conn.Export(nil, path, IwdSignalLevelAgentIface)
	}
	return unexport, nil
}

// signalLevelAgentObject is the unexported D-Bus object that implements
// net.connman.iwd.SignalLevelAgent by dispatching to a SignalLevelAgentHandler.
// Its exported methods match iwd's SignalLevelAgent interface signatures.
type signalLevelAgentObject struct {
	handler SignalLevelAgentHandler
}

// Changed implements net.connman.iwd.SignalLevelAgent.Changed.
func (a *signalLevelAgentObject) Changed(device dbus.ObjectPath, level uint8) *dbus.Error {
	if a.handler.Changed != nil {
		a.handler.Changed(device, level)
	}
	return nil
}

// Release implements net.connman.iwd.SignalLevelAgent.Release.
func (a *signalLevelAgentObject) Release() *dbus.Error {
	if a.handler.Release != nil {
		a.handler.Release()
	}
	return nil
}
