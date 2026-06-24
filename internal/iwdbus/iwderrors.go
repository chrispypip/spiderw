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
