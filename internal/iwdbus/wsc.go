package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdSimpleConfigurationIface is the fully qualified D-Bus interface name for
// iwd's WSC (Wi-Fi Simple Configuration, formerly WPS) support. It is exported on
// a device object when the device is in station mode, so a SimpleConfiguration
// shares its object path with the Device and Station. (iwd also exports it on P2P
// peer objects; spiderw uses the device-object form.)
const IwdSimpleConfigurationIface = IwdService + ".SimpleConfiguration"

// SimpleConfiguration wraps an iwd SimpleConfiguration (WSC / WPS) object using
// runtime introspection. It enrolls the station into an access point via WSC,
// either in PushButton (PBC) or PIN mode.
type SimpleConfiguration struct {
	call caller
}

// NewSimpleConfiguration creates a SimpleConfiguration for the given iwd object
// path (a device path).
func NewSimpleConfiguration(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*SimpleConfiguration, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdSimpleConfigurationIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdSimpleConfigurationIface)
	}
	return &SimpleConfiguration{call: caller(intro)}, nil
}

// PushButton starts WSC (WPS) enrollment in PushButton (PBC) mode: iwd
// disconnects any current network, scans for an access point advertising active
// PushButton mode, and connects to it. It is long-running: the call blocks until
// enrollment succeeds or fails (up to the WPS walk time), so callers should pass a
// context with a suitable deadline. iwd returns matchable errors, including
// ErrWSCSessionOverlap (more than one access point in PushButton mode, so the
// target is ambiguous), ErrWSCWalkTimeExpired (none found in the walk time),
// ErrWSCNoCredentials, ErrWSCNotReachable, ErrBusy, and ErrFailed.
func (c *SimpleConfiguration) PushButton(ctx context.Context) error {
	if err := c.ensureInitialized(); err != nil {
		return WrapConnection("SimpleConfiguration.ensureInitialized", err)
	}

	if _, err := c.call.Call(ctx, IwdSimpleConfigurationIface, "PushButton"); err != nil {
		return wrapIwdMethod(IwdSimpleConfigurationIface, "PushButton", err)
	}
	return nil
}

// GeneratePin asks iwd to generate a random 8-digit WSC PIN with a valid check
// digit, suitable for display to a user and for use with StartPin.
func (c *SimpleConfiguration) GeneratePin(ctx context.Context) (string, error) {
	if err := c.ensureInitialized(); err != nil {
		return "", WrapConnection("SimpleConfiguration.ensureInitialized", err)
	}

	body, err := c.call.Call(ctx, IwdSimpleConfigurationIface, "GeneratePin")
	if err != nil {
		return "", wrapIwdMethod(IwdSimpleConfigurationIface, "GeneratePin", err)
	}

	var pin string
	if err := dbus.Store(body, &pin); err != nil {
		return "", WrapVariant("GeneratePin", fmt.Errorf("unexpected reply shape: %w", err))
	}
	return pin, nil
}

// StartPin starts WSC (WPS) enrollment in PIN mode using pin, which may be a
// user-provided PIN or one from GeneratePin. Like PushButton it is long-running
// and blocks until enrollment succeeds or fails. iwd returns matchable errors
// including ErrWSCTimeExpired (no access point in PIN mode found in the allotted
// time), ErrWSCNoCredentials, ErrWSCNotReachable, ErrInvalidFormat (a malformed
// PIN), ErrBusy, and ErrFailed.
func (c *SimpleConfiguration) StartPin(ctx context.Context, pin string) error {
	if err := c.ensureInitialized(); err != nil {
		return WrapConnection("SimpleConfiguration.ensureInitialized", err)
	}

	if _, err := c.call.Call(ctx, IwdSimpleConfigurationIface, "StartPin", pin); err != nil {
		return wrapIwdMethod(IwdSimpleConfigurationIface, "StartPin", err)
	}
	return nil
}

// Cancel aborts an in-progress WSC (WPS) operation started by PushButton or
// StartPin. It surfaces iwd's matchable errors, including NotConnected when there
// is no operation to cancel.
func (c *SimpleConfiguration) Cancel(ctx context.Context) error {
	if err := c.ensureInitialized(); err != nil {
		return WrapConnection("SimpleConfiguration.ensureInitialized", err)
	}

	if _, err := c.call.Call(ctx, IwdSimpleConfigurationIface, "Cancel"); err != nil {
		return wrapIwdMethod(IwdSimpleConfigurationIface, "Cancel", err)
	}
	return nil
}

// ensureInitialized verifies that c has been initialized by NewSimpleConfiguration.
func (c *SimpleConfiguration) ensureInitialized() error {
	if c.call == nil {
		return ErrSimpleConfigurationUninitialized
	}
	return nil
}
