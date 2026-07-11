package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// SimpleConfiguration is a handle to a station's WSC (Wi-Fi Simple Configuration,
// formerly WPS) enrollment interface, obtained from Station.SimpleConfiguration.
// It joins the station to an access point without a passphrase, via either
// PushButton (PBC) or a PIN.
type SimpleConfiguration struct {
	core core.SimpleConfigurationIface
}

func newSimpleConfiguration(c core.SimpleConfigurationIface) *SimpleConfiguration {
	if c == nil {
		return nil
	}
	return &SimpleConfiguration{core: c}
}

// PushButton starts WSC enrollment in PushButton (PBC) mode: press the WPS button
// on the access point, then call this within its walk window. It blocks until iwd
// reports the outcome, so pass a context with a deadline. If more than one access
// point is in PushButton mode the target is ambiguous and iwd returns an error
// matching ErrWSCSessionOverlap.
func (c *SimpleConfiguration) PushButton(ctx context.Context) error {
	const op = "SimpleConfiguration.PushButton"
	log := logging.FromContext(ctx)

	if c == nil || c.core == nil {
		log.Error(ctx, "simple configuration uninitialized", "op", op)
		return wrapPublicError(op, ErrInternal)
	}
	if err := c.core.PushButton(ctx); err != nil {
		log.Error(ctx, "WSC push button enrollment failed", "op", op, "err", err)
		return wrapPublicError(op, err)
	}
	log.Debug(ctx, "WSC push button enrollment completed", "op", op)
	return nil
}

// GeneratePin returns a fresh 8-digit WSC PIN (with a valid check digit) to enter
// at the access point's registrar, for use with StartPin.
func (c *SimpleConfiguration) GeneratePin(ctx context.Context) (string, error) {
	const op = "SimpleConfiguration.GeneratePin"
	log := logging.FromContext(ctx)

	if c == nil || c.core == nil {
		log.Error(ctx, "simple configuration uninitialized", "op", op)
		return "", wrapPublicError(op, ErrInternal)
	}
	pin, err := c.core.GeneratePin(ctx)
	if err != nil {
		log.Error(ctx, "WSC generate pin failed", "op", op, "err", err)
		return "", wrapPublicError(op, err)
	}
	return pin, nil
}

// StartPin starts WSC enrollment in PIN mode using pin (typically one from
// GeneratePin, entered at the access point's registrar). Spaces and hyphens are
// ignored; the PIN must be 4 or 8 digits, and iwd validates the 8-digit check
// digit (surfaced as an error matching ErrInvalidFormat). It blocks until iwd
// reports the outcome.
func (c *SimpleConfiguration) StartPin(ctx context.Context, pin string) error {
	const op = "SimpleConfiguration.StartPin"
	log := logging.FromContext(ctx)

	if c == nil || c.core == nil {
		log.Error(ctx, "simple configuration uninitialized", "op", op)
		return wrapPublicError(op, ErrInternal)
	}
	if err := c.core.StartPin(ctx, pin); err != nil {
		log.Error(ctx, "WSC pin enrollment failed", "op", op, "err", err)
		return wrapPublicError(op, err)
	}
	log.Debug(ctx, "WSC pin enrollment completed", "op", op)
	return nil
}

// Cancel aborts an in-progress PushButton or StartPin enrollment.
func (c *SimpleConfiguration) Cancel(ctx context.Context) error {
	const op = "SimpleConfiguration.Cancel"
	log := logging.FromContext(ctx)

	if c == nil || c.core == nil {
		log.Error(ctx, "simple configuration uninitialized", "op", op)
		return wrapPublicError(op, ErrInternal)
	}
	if err := c.core.Cancel(ctx); err != nil {
		log.Error(ctx, "WSC cancel failed", "op", op, "err", err)
		return wrapPublicError(op, err)
	}
	return nil
}

// withSimpleConfiguration attaches the wiring hook that constructs the WSC handle
// for this station. The Client sets it at construction; a station without it (for
// example a bare test station) cannot use WSC.
func (s *Station) withSimpleConfiguration(fn func(ctx context.Context, path string) (core.SimpleConfigurationIface, error)) *Station {
	if s != nil {
		s.newSimpleConfig = fn
	}
	return s
}

// SimpleConfiguration returns a handle to the station's WSC (WPS) enrollment
// interface, for joining an access point without a passphrase via PushButton or
// PIN. It is available only when the device is in station mode and the driver
// supports WSC.
func (s *Station) SimpleConfiguration(ctx context.Context) (*SimpleConfiguration, error) {
	const op = "Station.SimpleConfiguration"
	log := logging.FromContext(ctx)

	if s == nil {
		log.Error(ctx, "station uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	if s.newSimpleConfig == nil {
		log.Error(ctx, "station does not support WSC", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}

	coreConfig, err := s.newSimpleConfig(ctx, s.path)
	if err != nil {
		log.Error(ctx, "WSC unavailable", "op", op, "err", err)
		return nil, wrapPublicError(op, err)
	}

	pub := newSimpleConfiguration(coreConfig)
	if pub == nil {
		log.Error(ctx, "WSC wrapper unexpectedly nil", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	log.Debug(ctx, "WSC handle constructed", "op", op)
	return pub, nil
}
