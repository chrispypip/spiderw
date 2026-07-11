package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// simpleConfigurationRaw is the iwdbus backend the core SimpleConfiguration
// wraps. It mirrors the iwdbus.SimpleConfiguration methods so the concrete type
// satisfies it directly and tests can substitute a fake.
type simpleConfigurationRaw interface {
	PushButton(ctx context.Context) error
	GeneratePin(ctx context.Context) (string, error)
	StartPin(ctx context.Context, pin string) error
	Cancel(ctx context.Context) error
}

// SimpleConfigurationIface is the core WSC surface the connect and public layers
// depend on, so they can substitute the concrete wrapper in tests.
type SimpleConfigurationIface interface {
	PushButton(ctx context.Context) error
	GeneratePin(ctx context.Context) (string, error)
	StartPin(ctx context.Context, pin string) error
	Cancel(ctx context.Context) error
}

// SimpleConfiguration is the core-layer wrapper over iwd's WSC
// (SimpleConfiguration) enrollment interface. It normalizes the PIN input and
// classifies iwd's errors while leaving the WSC-specific matchable sentinels
// (ErrWSCSessionOverlap and friends) intact in the chain.
type SimpleConfiguration struct {
	raw simpleConfigurationRaw
}

// NewSimpleConfiguration wraps raw. It returns nil when raw is nil so the connect
// layer can treat a missing WSC interface as unavailable.
func NewSimpleConfiguration(raw simpleConfigurationRaw) *SimpleConfiguration {
	if raw == nil {
		return nil
	}
	return &SimpleConfiguration{raw: raw}
}

func (c *SimpleConfiguration) rawConfig(op string) (simpleConfigurationRaw, error) {
	if c == nil || c.raw == nil {
		return nil, WrapInvalidState(ResourceSimpleConfiguration, op, "simple configuration wrapper was nil", ErrSimpleConfigurationNotInitialized)
	}
	return c.raw, nil
}

// PushButton starts WSC enrollment in PushButton (PBC) mode. It blocks until iwd
// reports the outcome; ErrWSCSessionOverlap and the other WSC errors remain
// matchable in the returned error.
func (c *SimpleConfiguration) PushButton(ctx context.Context) error {
	const op = "SimpleConfiguration.PushButton"

	raw, err := c.rawConfig(op)
	if err != nil {
		return err
	}

	if err := raw.PushButton(ctx); err != nil {
		return WrapSimpleConfigurationUnavailable(op, "failed starting WSC PushButton enrollment", err)
	}
	return nil
}

// GeneratePin returns a fresh iwd-generated 8-digit WSC PIN (with a valid check
// digit) suitable for display and for use with StartPin.
func (c *SimpleConfiguration) GeneratePin(ctx context.Context) (string, error) {
	const op = "SimpleConfiguration.GeneratePin"

	raw, err := c.rawConfig(op)
	if err != nil {
		return "", err
	}

	pin, err := raw.GeneratePin(ctx)
	if err != nil {
		return "", WrapSimpleConfigurationUnavailable(op, "failed generating WSC PIN", err)
	}
	return pin, nil
}

// StartPin starts WSC enrollment in PIN mode. The pin may include spaces or
// hyphens (as printed on device labels); StartPin strips them and validates the
// result is 4 or 8 digits before calling iwd, which validates the WSC check digit
// itself (surfaced as ErrInvalidFormat).
func (c *SimpleConfiguration) StartPin(ctx context.Context, pin string) error {
	const op = "SimpleConfiguration.StartPin"

	raw, err := c.rawConfig(op)
	if err != nil {
		return err
	}

	normalized, err := normalizeWSCPin(pin)
	if err != nil {
		return WrapInvalidArgument(ResourceSimpleConfiguration, op, err.Error(), ErrCore)
	}

	if err := raw.StartPin(ctx, normalized); err != nil {
		return WrapSimpleConfigurationUnavailable(op, "failed starting WSC PIN enrollment", err)
	}
	return nil
}

// Cancel aborts an in-progress WSC PushButton or PIN operation.
func (c *SimpleConfiguration) Cancel(ctx context.Context) error {
	const op = "SimpleConfiguration.Cancel"

	raw, err := c.rawConfig(op)
	if err != nil {
		return err
	}

	if err := raw.Cancel(ctx); err != nil {
		return WrapSimpleConfigurationUnavailable(op, "failed canceling WSC operation", err)
	}
	return nil
}

// normalizeWSCPin strips common label separators (spaces, hyphens) and checks the
// result is a 4- or 8-digit numeric WSC PIN. iwd validates the 8-digit check
// digit itself.
func normalizeWSCPin(pin string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, pin)

	if cleaned == "" {
		return "", errors.New("WSC PIN must not be empty")
	}
	for _, r := range cleaned {
		if r < '0' || r > '9' {
			return "", errors.New("WSC PIN must contain only digits (optionally spaced or hyphenated)")
		}
	}
	if len(cleaned) != 4 && len(cleaned) != 8 {
		return "", fmt.Errorf("WSC PIN must be 4 or 8 digits, got %d", len(cleaned))
	}
	return cleaned, nil
}
