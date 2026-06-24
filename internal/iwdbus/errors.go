package iwdbus

import (
	"errors"
	"fmt"
)

// Error sentinels classify low-level D-Bus failures.
var (
	// ErrDBusConnection indicates fundamental connection issues.
	ErrDBusConnection = errors.New("dbus connection error")

	// ErrDBusMethod indicates a D-Bus method call failure.
	ErrDBusMethod = errors.New("dbus method error")

	// ErrDBusProperty indicates a D-Bus Get/Set property failure.
	ErrDBusProperty = errors.New("dbus property error")

	// ErrDBusIntrospection indicates an introspection/interface discovery
	// failure.
	ErrDBusIntrospection = errors.New("dbus introspection error")

	// ErrDBusVariant indicates failure converting or parsing D-Bus variant values.
	ErrDBusVariant = errors.New("dbus variant conversion error")

	// ErrDaemonUninitialized indicates that a Daemon was used before initialization.
	ErrDaemonUninitialized = errors.New("daemon not initialized")

	// ErrAdapterUninitialized indicates that an Adapter was used before initialization.
	ErrAdapterUninitialized = errors.New("adapter is not initialized")

	// ErrDeviceUninitialized indicates that a Device was used before initialization.
	ErrDeviceUninitialized = errors.New("device is not initialized")

	// ErrBasicServiceSetUninitialized indicates that a BasicServiceSet was used
	// before initialization.
	ErrBasicServiceSetUninitialized = errors.New("basic service set is not initialized")

	// ErrNetworkUninitialized indicates that a Network was used before
	// initialization.
	ErrNetworkUninitialized = errors.New("network is not initialized")

	// ErrNoAgent indicates that iwd rejected an operation because no credentials
	// agent is registered. Connecting to a secured network that is not already
	// known requires a registered agent to supply credentials.
	ErrNoAgent = errors.New("no credentials agent registered")
)

// Error is the internal structured error type used for all D-Bus-layer failures.
//
// It wraps:
//   - Kind:   the sentinel error category (ErrDBusMethod, ErrDBusProperty, etc.)
//   - Context: descriptive metadata about the operation
//   - Err:     the underlying D-Bus or lower-level error
//
// This enables errors.Is(err, ErrDBusMethod) to work.
type Error struct {
	Kind    error
	Context string
	Err     error
}

// Error returns a human-readable low-level D-Bus error string.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Kind.Error(), e.Context, e.Err)
}

// Unwrap returns the sentinel and underlying errors for errors.Is, errors.As,
// and errors.AsType.
func (e *Error) Unwrap() error {
	return errors.Join(e.Kind, e.Err)
}

// -----------------------------------------------------------------------------
// Wrapper construction helpers
// -----------------------------------------------------------------------------

// WrapConnection standardizes connection-related errors.
func WrapConnection(op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:    ErrDBusConnection,
		Context: "op=" + op,
		Err:     err,
	}
}

// WrapMethod standardizes D-Bus method failure errors.
func WrapMethod(iface, method string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:    ErrDBusMethod,
		Context: fmt.Sprintf("iface=%s, method=%s", iface, method),
		Err:     err,
	}
}

// WrapProperty standardizes D-Bus property failure errors.
func WrapProperty(iface, property string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:    ErrDBusProperty,
		Context: fmt.Sprintf("iface=%s, property=%s", iface, property),
		Err:     err,
	}
}

// WrapIntrospection standardizes D-Bus introspection failure errors.
func WrapIntrospection(path string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:    ErrDBusIntrospection,
		Context: "path=" + path,
		Err:     err,
	}
}

// WrapVariant standardizes D-Bus variant conversion failure errors.
func WrapVariant(name string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:    ErrDBusVariant,
		Context: "variant=" + name,
		Err:     err,
	}
}
