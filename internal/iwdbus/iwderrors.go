package iwdbus

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// iwd D-Bus error names returned by methods such as Network.Connect. These are
// the fully-qualified names iwd places in the D-Bus error reply.
const (
	// IwdErrorNoAgent is returned when no credentials agent is registered.
	IwdErrorNoAgent = IwdService + ".NoAgent"

	// IwdErrorAborted is returned when an operation was aborted.
	IwdErrorAborted = IwdService + ".Aborted"

	// IwdErrorBusy is returned when iwd is busy with another operation.
	IwdErrorBusy = IwdService + ".Busy"

	// IwdErrorFailed is returned for a generic operation failure.
	IwdErrorFailed = IwdService + ".Failed"

	// IwdErrorNotSupported is returned when an operation is not supported.
	IwdErrorNotSupported = IwdService + ".NotSupported"

	// IwdErrorTimeout is returned when an operation timed out.
	IwdErrorTimeout = IwdService + ".Timeout"

	// IwdErrorInProgress is returned when the operation is already in progress.
	IwdErrorInProgress = IwdService + ".InProgress"

	// IwdErrorNotConfigured is returned when the network is not configured.
	IwdErrorNotConfigured = IwdService + ".NotConfigured"

	// IwdErrorNotFound is returned when a referenced object does not exist (for
	// example unregistering an agent that was never registered).
	IwdErrorNotFound = IwdService + ".NotFound"

	// IwdErrorAlreadyExists is returned when an object already exists (for
	// example registering a second agent on a connection that already has one).
	IwdErrorAlreadyExists = IwdService + ".AlreadyExists"

	// IwdErrorInvalidArguments is returned when a method receives invalid
	// arguments.
	IwdErrorInvalidArguments = IwdService + ".InvalidArguments"

	// IwdErrorInvalidFormat is returned when an argument has an invalid format.
	IwdErrorInvalidFormat = IwdService + ".InvalidFormat"

	// IwdErrorNotConnected is returned when an operation requires a connection
	// that does not exist.
	IwdErrorNotConnected = IwdService + ".NotConnected"

	// IwdErrorNotImplemented is returned when a method is not implemented.
	IwdErrorNotImplemented = IwdService + ".NotImplemented"

	// IwdErrorServiceSetOverlap is returned when service sets overlap.
	IwdErrorServiceSetOverlap = IwdService + ".ServiceSetOverlap"

	// IwdErrorAlreadyProvisioned is returned when a network is already
	// provisioned.
	IwdErrorAlreadyProvisioned = IwdService + ".AlreadyProvisioned"

	// IwdErrorNotHidden is returned when an operation expects a hidden network
	// but the target is not hidden.
	IwdErrorNotHidden = IwdService + ".NotHidden"

	// IwdErrorNotAvailable is returned when a requested operation is not
	// currently available.
	IwdErrorNotAvailable = IwdService + ".NotAvailable"

	// IwdErrorPermissionDenied is returned when the caller lacks permission for a
	// requested operation.
	IwdErrorPermissionDenied = IwdService + ".PermissionDenied"
)

// Sentinels for iwd-reported failures. They remain matchable with errors.Is
// through the core and public error chains, so callers can react to a specific
// iwd outcome (for example, retry on ErrBusy/ErrInProgress, give up on
// ErrNotSupported) without parsing error text.
//
// ErrNoAgent is declared in errors.go alongside the other iwdbus sentinels.
var (
	// ErrAborted matches net.connman.iwd.Aborted.
	ErrAborted = errors.New("iwd operation aborted")

	// ErrBusy matches net.connman.iwd.Busy.
	ErrBusy = errors.New("iwd is busy")

	// ErrFailed matches net.connman.iwd.Failed.
	ErrFailed = errors.New("iwd operation failed")

	// ErrNotSupported matches net.connman.iwd.NotSupported.
	ErrNotSupported = errors.New("iwd operation not supported")

	// ErrTimeout matches net.connman.iwd.Timeout.
	ErrTimeout = errors.New("iwd operation timed out")

	// ErrInProgress matches net.connman.iwd.InProgress.
	ErrInProgress = errors.New("iwd operation already in progress")

	// ErrNotConfigured matches net.connman.iwd.NotConfigured.
	ErrNotConfigured = errors.New("iwd network not configured")

	// ErrNotFound matches net.connman.iwd.NotFound.
	ErrNotFound = errors.New("iwd object not found")

	// ErrAlreadyExists matches net.connman.iwd.AlreadyExists.
	ErrAlreadyExists = errors.New("iwd object already exists")

	// ErrInvalidArguments matches net.connman.iwd.InvalidArguments.
	ErrInvalidArguments = errors.New("iwd invalid arguments")

	// ErrInvalidFormat matches net.connman.iwd.InvalidFormat.
	ErrInvalidFormat = errors.New("iwd invalid format")

	// ErrNotConnected matches net.connman.iwd.NotConnected.
	ErrNotConnected = errors.New("iwd not connected")

	// ErrNotImplemented matches net.connman.iwd.NotImplemented.
	ErrNotImplemented = errors.New("iwd operation not implemented")

	// ErrServiceSetOverlap matches net.connman.iwd.ServiceSetOverlap.
	ErrServiceSetOverlap = errors.New("iwd service set overlap")

	// ErrAlreadyProvisioned matches net.connman.iwd.AlreadyProvisioned.
	ErrAlreadyProvisioned = errors.New("iwd network already provisioned")

	// ErrNotHidden matches net.connman.iwd.NotHidden.
	ErrNotHidden = errors.New("iwd network not hidden")

	// ErrNotAvailable matches net.connman.iwd.NotAvailable.
	ErrNotAvailable = errors.New("iwd operation not available")

	// ErrPermissionDenied matches net.connman.iwd.PermissionDenied.
	ErrPermissionDenied = errors.New("iwd permission denied")
)

// iwdErrorSentinels maps a recognized iwd D-Bus error name to its sentinel.
var iwdErrorSentinels = map[string]error{
	IwdErrorNoAgent:       ErrNoAgent,
	IwdErrorAborted:       ErrAborted,
	IwdErrorBusy:          ErrBusy,
	IwdErrorFailed:        ErrFailed,
	IwdErrorNotSupported:  ErrNotSupported,
	IwdErrorTimeout:       ErrTimeout,
	IwdErrorInProgress:    ErrInProgress,
	IwdErrorNotConfigured: ErrNotConfigured,

	IwdErrorNotFound:           ErrNotFound,
	IwdErrorAlreadyExists:      ErrAlreadyExists,
	IwdErrorInvalidArguments:   ErrInvalidArguments,
	IwdErrorInvalidFormat:      ErrInvalidFormat,
	IwdErrorNotConnected:       ErrNotConnected,
	IwdErrorNotImplemented:     ErrNotImplemented,
	IwdErrorServiceSetOverlap:  ErrServiceSetOverlap,
	IwdErrorAlreadyProvisioned: ErrAlreadyProvisioned,
	IwdErrorNotHidden:          ErrNotHidden,
	IwdErrorNotAvailable:       ErrNotAvailable,
	IwdErrorPermissionDenied:   ErrPermissionDenied,
}

// iwdErrorSentinel returns the sentinel for a recognized iwd D-Bus error, or nil
// when err is not a recognized iwd error.
func iwdErrorSentinel(err error) error {
	var de dbus.Error
	if errors.As(err, &de) {
		if sentinel, ok := iwdErrorSentinels[de.Name]; ok {
			return sentinel
		}
	}
	return nil
}

// wrapIwdMethod wraps a D-Bus method-call error, mapping recognized iwd error
// names to their sentinels so callers can match them with errors.Is. The
// original D-Bus error is preserved in the chain for diagnostics. Unrecognized
// errors fall back to a generic method error.
func wrapIwdMethod(iface, method string, err error) error {
	if err == nil {
		return nil
	}
	if sentinel := iwdErrorSentinel(err); sentinel != nil {
		return &Error{
			Kind:    ErrDBusMethod,
			Context: fmt.Sprintf("iface=%s, method=%s", iface, method),
			Err:     fmt.Errorf("%w: %w", sentinel, err),
		}
	}
	return WrapMethod(iface, method, err)
}

// wrapIwdProperty wraps a D-Bus property Get/Set error, mapping recognized iwd
// error names to their sentinels so callers can match them with errors.Is (e.g.
// a writable property that hardware rejects with NotSupported). The original
// D-Bus error is preserved in the chain. Unrecognized errors fall back to a
// generic property error.
func wrapIwdProperty(iface, property string, err error) error {
	if err == nil {
		return nil
	}
	if sentinel := iwdErrorSentinel(err); sentinel != nil {
		return &Error{
			Kind:    ErrDBusProperty,
			Context: fmt.Sprintf("iface=%s, property=%s", iface, property),
			Err:     fmt.Errorf("%w: %w", sentinel, err),
		}
	}
	return WrapProperty(iface, property, err)
}
