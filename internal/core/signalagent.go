package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// SignalLevelConfig holds the normalized configuration for a signal-level agent:
// the RSSI thresholds that trigger notifications and the callbacks iwd's calls
// are forwarded to.
type SignalLevelConfig struct {
	// Thresholds are RSSI levels in dBm. iwd calls Changed whenever the connected
	// network's signal crosses one of them. They must be non-empty and in
	// strictly descending order (iwd's requirement), each within int16 range.
	Thresholds []int

	// Changed is called with the new signal band index whenever the signal
	// crosses a threshold: 0 is the strongest band (stronger than the first
	// threshold) and higher values are progressively weaker. Required.
	Changed func(level int)

	// Released is called when iwd releases the agent (it was unregistered or the
	// station went away). Optional.
	Released func()
}

// Validate reports whether c is a usable signal-level configuration, turning
// otherwise-silent misconfiguration into an explicit invalid-argument error.
func (c SignalLevelConfig) Validate(op string) error {
	if c.Changed == nil {
		return WrapInvalidArgument(ResourceStation, op, "Changed callback must be set", ErrCore)
	}
	if len(c.Thresholds) == 0 {
		return WrapInvalidArgument(ResourceStation, op, "at least one signal-strength threshold is required", ErrCore)
	}
	for i, t := range c.Thresholds {
		if t < math.MinInt16 || t > math.MaxInt16 {
			return WrapInvalidArgument(ResourceStation, op, fmt.Sprintf("signal threshold %d dBm is out of range", t), ErrCore)
		}
		if i > 0 && t >= c.Thresholds[i-1] {
			return WrapInvalidArgument(ResourceStation, op, "signal thresholds must be in strictly descending order", ErrCore)
		}
	}
	return nil
}

// signalLevelRegistrarRaw is the raw backend used to tear a signal-level agent
// down. It mirrors the relevant iwdbus.Station method so the concrete type
// satisfies it directly. Registration is driven by the connect layer; the core
// agent only owns unregistration.
type signalLevelRegistrarRaw interface {
	UnregisterSignalLevelAgent(ctx context.Context, path dbus.ObjectPath) error
}

// SignalLevelAgentIface defines the core signal-level agent operations used by
// the public layer.
type SignalLevelAgentIface interface {
	Unregister(ctx context.Context) error
}

// SignalLevelAgent is the core-layer handle for a registered signal-level agent.
// It normalizes iwd's Changed/Release calls into the configured callbacks and
// owns the cleanup (UnregisterSignalLevelAgent + unexport) run by Unregister.
//
// The callback and threshold fields are set once at construction and only read
// afterward; the registration state guarded by mu is what Bind and Unregister
// mutate.
type SignalLevelAgent struct {
	changed  func(level int)
	released func()
	levels   []int16

	mu         sync.Mutex
	registered bool
	registrar  signalLevelRegistrarRaw
	path       dbus.ObjectPath
	unexport   func() error
}

// NewSignalLevelAgent creates an unregistered core SignalLevelAgent from cfg and
// returns it along with the iwdbus.SignalLevelAgentHandler that forwards iwd's
// calls to cfg's callbacks. Callers should Validate cfg first; New converts the
// thresholds assuming they are already in range. The connect layer exports the
// handler, registers it through a station using Levels(), then calls Bind.
func NewSignalLevelAgent(cfg SignalLevelConfig) (*SignalLevelAgent, iwdbus.SignalLevelAgentHandler) {
	levels := make([]int16, len(cfg.Thresholds))
	for i, t := range cfg.Thresholds {
		levels[i] = int16(t)
	}
	a := &SignalLevelAgent{
		changed:  cfg.Changed,
		released: cfg.Released,
		levels:   levels,
	}
	return a, a.buildHandler()
}

// Levels returns a copy of the validated RSSI thresholds, in the int16 form iwd
// expects, for the connect layer to register with.
func (a *SignalLevelAgent) Levels() []int16 {
	out := make([]int16, len(a.levels))
	copy(out, a.levels)
	return out
}

// Bind attaches the station registrar, agent object path, and unexport cleanup
// so Unregister can tear the agent down. The connect layer calls Bind after a
// successful RegisterSignalLevelAgent.
func (a *SignalLevelAgent) Bind(registrar signalLevelRegistrarRaw, path dbus.ObjectPath, unexport func() error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.registrar = registrar
	a.path = path
	a.unexport = unexport
	a.registered = true
}

// Unregister unregisters the agent from its station and removes the exported
// object. It is idempotent and nil-safe.
func (a *SignalLevelAgent) Unregister(ctx context.Context) error {
	const op = "SignalLevelAgent.Unregister"

	if a == nil {
		return WrapInvalidState(ResourceStation, op, "signal level agent wrapper was nil", ErrSignalLevelAgentNotInitialized)
	}

	a.mu.Lock()
	if !a.registered {
		a.mu.Unlock()
		return nil
	}
	a.registered = false
	registrar := a.registrar
	path := a.path
	unexport := a.unexport
	a.mu.Unlock()

	var unregErr error
	if registrar != nil {
		if err := registrar.UnregisterSignalLevelAgent(ctx, path); err != nil {
			unregErr = WrapStationUnavailable(op, "failed unregistering signal level agent", err)
		}
	}

	var unexportErr error
	if unexport != nil {
		if err := unexport(); err != nil {
			unexportErr = WrapStationUnavailable(op, "failed unexporting signal level agent object", err)
		}
	}

	return errors.Join(unregErr, unexportErr)
}

// buildHandler returns the iwdbus.SignalLevelAgentHandler that forwards iwd's
// notifications to the configured callbacks. Both guards are defensive; Validate
// already requires Changed to be set.
func (a *SignalLevelAgent) buildHandler() iwdbus.SignalLevelAgentHandler {
	return iwdbus.SignalLevelAgentHandler{
		Changed: func(device dbus.ObjectPath, level uint8) {
			if a.changed != nil {
				a.changed(int(level))
			}
		},
		Release: a.handleRelease,
	}
}

func (a *SignalLevelAgent) handleRelease() {
	if a.released != nil {
		a.released()
	}
}
