package mock

import (
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// initialSignalLevel is the band index the mock reports via Changed shortly
// after a signal-level agent registers, simulating iwd delivering the current
// signal strength. Integration tests assert this value.
const initialSignalLevel uint8 = 1

// signalLevelReg records one registered signal-level agent.
type signalLevelReg struct {
	sender string
	path   dbus.ObjectPath
	levels []int16
}

// signalLevelAgentRegistry tracks signal-level agents registered per station via
// Station.RegisterSignalLevelAgent and calls back into them, mirroring iwd's
// one-agent-per-station model.
type signalLevelAgentRegistry struct {
	mu        sync.Mutex
	conn      *dbus.Conn
	byStation map[dbus.ObjectPath]signalLevelReg
}

var signalAgents = signalLevelAgentRegistry{byStation: map[dbus.ObjectPath]signalLevelReg{}}

// bindConn records the connection used to call back into a registered agent.
func (r *signalLevelAgentRegistry) bindConn(conn *dbus.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conn = conn
}

// register stores the agent for station, mirroring iwd's AlreadyExists rejection
// of a second agent on the same station, then simulates an initial Changed.
func (r *signalLevelAgentRegistry) register(sender dbus.Sender, station, agentPath dbus.ObjectPath, levels []int16) *dbus.Error {
	r.mu.Lock()
	if _, ok := r.byStation[station]; ok {
		r.mu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorAlreadyExists, []interface{}{"a signal level agent is already registered"})
	}
	r.byStation[station] = signalLevelReg{sender: string(sender), path: agentPath, levels: levels}
	r.mu.Unlock()

	// Deliver an initial band crossing from a goroutine so we do not re-enter the
	// client's in-flight Register call. The client's agent object is already
	// exported (export precedes register), so it is ready to receive Changed.
	go r.emitChanged(station, initialSignalLevel)
	return nil
}

// unregister clears the agent for station. It mirrors iwd's NotFound rejection
// when the path was never registered there, or by a different client (sender).
func (r *signalLevelAgentRegistry) unregister(sender dbus.Sender, station, agentPath dbus.ObjectPath) *dbus.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	reg, ok := r.byStation[station]
	if !ok || reg.path != agentPath || reg.sender != string(sender) {
		return dbus.NewError(iwdbus.IwdErrorNotFound, []interface{}{"no such signal level agent registered"})
	}
	delete(r.byStation, station)
	return nil
}

// emitChanged calls the registered agent's Changed(device, level) for station,
// a no-op if no agent is registered there.
func (r *signalLevelAgentRegistry) emitChanged(station dbus.ObjectPath, level uint8) {
	r.mu.Lock()
	conn := r.conn
	reg, ok := r.byStation[station]
	r.mu.Unlock()
	if !ok || conn == nil {
		return
	}
	conn.Object(reg.sender, reg.path).Call(iwdbus.IwdSignalLevelAgentIface+".Changed", dbus.FlagNoReplyExpected, station, level)
}

// RegisterSignalLevelAgent implements Station.RegisterSignalLevelAgent. godbus
// injects sender; the wire arguments are the agent object path and thresholds.
func (d *Device) RegisterSignalLevelAgent(sender dbus.Sender, agentPath dbus.ObjectPath, levels []int16) *dbus.Error {
	return signalAgents.register(sender, d.Path, agentPath, levels)
}

// UnregisterSignalLevelAgent implements Station.UnregisterSignalLevelAgent.
func (d *Device) UnregisterSignalLevelAgent(sender dbus.Sender, agentPath dbus.ObjectPath) *dbus.Error {
	return signalAgents.unregister(sender, d.Path, agentPath)
}
