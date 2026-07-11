package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// SignalLevelConfig configures signal-strength monitoring for a station. iwd
// invokes Changed whenever the connected network's RSSI crosses one of the
// configured thresholds.
type SignalLevelConfig struct {
	// Thresholds are RSSI levels in dBm that trigger Changed when crossed. They
	// must be non-empty and in strictly descending order (for example
	// []int{-60, -70, -80}). Required.
	Thresholds []int

	// Changed is called with the signal band index each time the connected
	// network's RSSI crosses a threshold. N thresholds define N+1 bands, so level
	// ranges over [0, N]: 0 is the strongest band (above the first threshold) and
	// N is the weakest (below the last). Required.
	//
	// The band is derived entirely by iwd from its own RSSI measurement, which is
	// averaged and driver-dependent. It will not match an instantaneous reading
	// such as "iw dev link" exactly: transitions can occur several dBm away from
	// the nominal thresholds, more so while the signal is actively changing. iwd
	// reports a band only when the index changes, so a band the signal passes
	// through quickly may be skipped, and some drivers (notably brcmfmac) support
	// only coarse threshold monitoring. Treat level as a coarse, hysteresis-
	// smoothed indicator rather than an exact dBm boundary.
	Changed func(level int)

	// Released is called when iwd releases the agent (the monitor was
	// unregistered or the station went away). Optional.
	Released func()
}

func (c SignalLevelConfig) toCore() core.SignalLevelConfig {
	return core.SignalLevelConfig{
		Thresholds: c.Thresholds,
		Changed:    c.Changed,
		Released:   c.Released,
	}
}

// SignalLevelAgent is a handle to an active signal-level monitor registered on a
// station. Call Unregister to stop monitoring and release it.
type SignalLevelAgent struct {
	core core.SignalLevelAgentIface
}

func newSignalLevelAgent(c core.SignalLevelAgentIface) *SignalLevelAgent {
	if c == nil {
		return nil
	}
	return &SignalLevelAgent{core: c}
}

// Unregister stops the monitor: it unregisters the agent from iwd and removes the
// exported object. It is idempotent.
func (a *SignalLevelAgent) Unregister(ctx context.Context) error {
	const op = "SignalLevelAgent.Unregister"
	log := logging.FromContext(ctx)

	if a == nil || a.core == nil {
		log.Error(ctx, "signal level agent uninitialized", "op", op)
		return wrapPublicError(op, ErrInternal)
	}
	if err := a.core.Unregister(ctx); err != nil {
		log.Error(ctx, "signal level agent unregister failed", "op", op, "err", err)
		return wrapPublicError(op, err)
	}
	log.Debug(ctx, "signal level agent unregistered", "op", op)
	return nil
}

// withSignalMonitor attaches the wiring hook that registers a signal-level agent
// for this station. The Client sets it at construction; a station without it
// (for example a bare test station) cannot monitor signal level.
func (s *Station) withSignalMonitor(fn func(ctx context.Context, stationPath string, cfg core.SignalLevelConfig) (core.SignalLevelAgentIface, error)) *Station {
	if s != nil {
		s.registerSignalAgent = fn
	}
	return s
}

// MonitorSignalLevel starts monitoring the station's connected-network signal
// strength, invoking cfg.Changed whenever the RSSI crosses one of cfg.Thresholds
// (dBm, strictly descending). It returns a handle whose Unregister stops
// monitoring. iwd supports a single signal-level monitor per station.
func (s *Station) MonitorSignalLevel(ctx context.Context, cfg SignalLevelConfig) (*SignalLevelAgent, error) {
	const op = "Station.MonitorSignalLevel"
	log := logging.FromContext(ctx)

	if s == nil {
		log.Error(ctx, "station uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	cc := cfg.toCore()
	if err := cc.Validate(op); err != nil {
		log.Error(ctx, "invalid signal level config", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	if s.registerSignalAgent == nil {
		log.Error(ctx, "station does not support signal monitoring", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreAgent, err := s.registerSignalAgent(ctx, s.path, cc)
	if err != nil {
		log.Error(ctx, "signal level monitor registration failed", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newSignalLevelAgent(coreAgent)
	if pub == nil {
		log.Error(ctx, "signal level agent wrapper unexpectedly nil", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	log.Debug(ctx, "signal level monitor registered", "op", op)
	return pub, nil
}
