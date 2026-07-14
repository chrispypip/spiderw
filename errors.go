package spiderw

import (
	"errors"
	"fmt"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/failure"
)

// -----------------------------------------------------------------------------
// Public-facing error kinds
// -----------------------------------------------------------------------------
//
// These are semantic categories stable across versions of spiderw.
// External callers should rely on these for handling structured errors.
//

// Kind identifies a stable public spiderw error category.
type Kind string

// Kind constants are stable public error categories.
const (
	// KindUnavailable indicates that the requested resource or subsystem could
	// not be reached or did not expose the expected API.
	KindUnavailable Kind = Kind(failure.KindUnavailable)

	// KindInvalidState indicates that spiderw observed an invalid or
	// inconsistent state from iwd or its own wrappers.
	KindInvalidState Kind = Kind(failure.KindInvalidState)

	// KindInvalidArgument indicates that a caller supplied an invalid argument
	// to the public API.
	KindInvalidArgument Kind = Kind(failure.KindInvalidArgument)

	// KindInternal indicates an uncategorized internal spiderw failure.
	KindInternal Kind = Kind(failure.KindInternal)
)

// Resource identifies which public spiderw object or subsystem an error applies to.
type Resource string

// Resource constants classify public errors by target object/subsystem.
const (
	// ResourceUnknown indicates that no specific resource is known.
	ResourceUnknown Resource = Resource(failure.ResourceUnknown)

	// ResourceClient identifies client-level failures.
	ResourceClient Resource = Resource(failure.ResourceClient)

	// ResourceDaemon identifies failures involving the iwd daemon object.
	ResourceDaemon Resource = Resource(failure.ResourceDaemon)

	// ResourceAdapter identifies failures involving an iwd adapter object.
	ResourceAdapter Resource = Resource(failure.ResourceAdapter)

	// ResourceDevice identifies failures involving an iwd device object.
	ResourceDevice Resource = Resource(failure.ResourceDevice)

	// ResourceBasicServiceSet identifies failures involving an iwd basic service
	// set (BSS) object.
	ResourceBasicServiceSet Resource = Resource(failure.ResourceBasicServiceSet)

	// ResourceStation identifies failures involving an iwd station object.
	ResourceStation Resource = Resource(failure.ResourceStation)

	// ResourceSimpleConfiguration identifies failures involving iwd's WSC (Wi-Fi
	// Simple Configuration / WPS) interface, reached via Station.SimpleConfiguration.
	ResourceSimpleConfiguration Resource = Resource(failure.ResourceSimpleConfiguration)

	// ResourceNetwork identifies failures involving an iwd network object.
	ResourceNetwork Resource = Resource(failure.ResourceNetwork)

	// ResourceKnownNetwork identifies failures involving an iwd known-network
	// object.
	ResourceKnownNetwork Resource = Resource(failure.ResourceKnownNetwork)

	// ResourceAgent identifies failures involving the iwd credentials agent or
	// agent manager.
	ResourceAgent Resource = Resource(failure.ResourceAgent)

	// ResourceAccessPoint identifies failures involving an iwd access point (a
	// device running in AP mode).
	ResourceAccessPoint Resource = Resource(failure.ResourceAccessPoint)
)

// -----------------------------------------------------------------------------
// Public error sentinels for errors.Is
// -----------------------------------------------------------------------------
//
// These are the *correct* way for public consumers to test error categories:
//
//    if errors.Is(err, spiderw.ErrUnavailable) { ... }
//
// They behave similarly to os.ErrNotExist and other Go stdlib practice.
//

// Error sentinels support errors.Is checks against public error categories.
var (
	// ErrUnavailable matches errors whose public kind is KindUnavailable.
	ErrUnavailable = errors.New("unavailable")

	// ErrInvalidState matches errors whose public kind is KindInvalidState.
	ErrInvalidState = errors.New("invalid state")

	// ErrInternal matches errors whose public kind is KindInternal.
	ErrInternal = errors.New("internal error")

	// ErrInvalidArgument matches errors whose public kind is KindInvalidArgument.
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrSpiderw matches all structured errors returned by the public API.
	ErrSpiderw = errors.New("spiderw api error")

	// ErrNoAgent matches errors caused by iwd rejecting an operation because no
	// credentials agent is registered. Connecting to a secured network that is
	// not already known requires a registered agent; until then, Network.Connect
	// returns an error matching ErrNoAgent.
	ErrNoAgent = core.ErrNoAgent

	// The following sentinels match named iwd D-Bus errors surfaced by operations
	// such as Network.Connect. Use errors.Is to react to a specific iwd outcome
	// (for example, retry on ErrInProgress, give up on ErrNotSupported) without
	// parsing error text. Note: current iwd reports a busy/in-progress condition
	// as ErrInProgress; ErrBusy and ErrTimeout are retained for compatibility but
	// are not emitted by iwd today.
	ErrAborted       = core.ErrAborted
	ErrBusy          = core.ErrBusy
	ErrFailed        = core.ErrFailed
	ErrNotSupported  = core.ErrNotSupported
	ErrTimeout       = core.ErrTimeout
	ErrInProgress    = core.ErrInProgress
	ErrNotConfigured = core.ErrNotConfigured

	// ErrNotFound and ErrAlreadyExists are surfaced by the agent manager
	// (UnregisterAgent on an unregistered agent; RegisterAgent when another agent
	// already owns the connection), among other operations.
	ErrNotFound      = core.ErrNotFound
	ErrAlreadyExists = core.ErrAlreadyExists
	// ErrInvalidArguments matches iwd's named net.connman.iwd.InvalidArguments
	// error. It is distinct from ErrInvalidArgument (singular), which matches any
	// public KindInvalidArgument error spiderw itself raises.
	ErrInvalidArguments   = core.ErrInvalidArguments
	ErrInvalidFormat      = core.ErrInvalidFormat
	ErrNotConnected       = core.ErrNotConnected
	ErrNotImplemented     = core.ErrNotImplemented
	ErrServiceSetOverlap  = core.ErrServiceSetOverlap
	ErrAlreadyProvisioned = core.ErrAlreadyProvisioned
	ErrNotHidden          = core.ErrNotHidden
	ErrNotAvailable       = core.ErrNotAvailable
	// ErrPermissionDenied matches iwd's net.connman.iwd.PermissionDenied, returned
	// when the caller lacks permission for a privileged operation.
	ErrPermissionDenied = core.ErrPermissionDenied

	// The following match iwd's WSC (SimpleConfiguration) enrollment errors, so a
	// caller can react to a specific WSC outcome with errors.Is. ErrWSCSessionOverlap
	// means more than one access point was in PushButton mode; ErrWSCWalkTimeExpired
	// and ErrWSCTimeExpired mean no access point was found in PushButton / PIN mode
	// within the allotted time.
	ErrWSCSessionOverlap  = core.ErrWSCSessionOverlap
	ErrWSCNoCredentials   = core.ErrWSCNoCredentials
	ErrWSCNotReachable    = core.ErrWSCNotReachable
	ErrWSCWalkTimeExpired = core.ErrWSCWalkTimeExpired
	ErrWSCTimeExpired     = core.ErrWSCTimeExpired
)

// -----------------------------------------------------------------------------
// Public API error type
// -----------------------------------------------------------------------------

// Error is the structured error type returned by the public API.
//
// Underlying core and D-Bus errors remain discoverable via errors.Is,
// errors.As, and errors.AsType.
//
// Example:
//
//	v, err := client.Daemon().Version(ctx)
//	if errors.Is(err, spiderw.ErrUnavailable) { ... }
type Error struct {
	Kind     Kind     // stable API category
	Resource Resource // public object/subsystem involved, when known
	Op       string   // public-facing operation name (for example, "Daemon.Version")
	Details  string   // optional human-friendly text
	Err      error    // wrapped core.Error or raw error
}

// Error returns a human-readable public API error string.
func (e *Error) Error() string {
	label := publicErrorLabel(e.Kind, e.Resource)

	// When we wrap a core.Error, its own Error() restates the same label, Op,
	// and Details we are about to print, which would duplicate every frame.
	// Render the core error's underlying cause instead; the core.Error itself
	// stays in the Unwrap chain for errors.Is / errors.As / errors.AsType.
	cause := e.Err
	if ce, ok := errors.AsType[*core.Error](e.Err); ok {
		cause = ce.Err
	}

	if e.Details != "" {
		return fmt.Sprintf("%s: Op=%s: %v (%s)", label, e.Op, cause, e.Details)
	}
	return fmt.Sprintf("%s: Op=%s: %v", label, e.Op, cause)
}

// Unwrap exposes:
//   - ErrSpiderw (public indication this came from spiderw)
//   - the sentinel for this Kind
//   - the underlying wrapped error
func (e *Error) Unwrap() error {
	return errors.Join(ErrSpiderw, sentinelForKind(e.Kind), e.Err)
}

// sentinelForKind returns the public sentinel error for the given Kind.
func sentinelForKind(k Kind) error {
	switch k {
	case KindUnavailable:
		return ErrUnavailable
	case KindInvalidState:
		return ErrInvalidState
	case KindInvalidArgument:
		return ErrInvalidArgument
	default:
		return ErrInternal
	}
}

// -----------------------------------------------------------------------------
// Mapping: core layer to public API kinds
// -----------------------------------------------------------------------------

func mapCoreKind(k core.Kind) Kind {
	return Kind(failure.Public(k))
}

func mapCoreResource(r core.Resource) Resource {
	return Resource(r)
}

// -----------------------------------------------------------------------------
// Public wrapper for client.go and other public entry points.
// -----------------------------------------------------------------------------

func wrapPublicError(op string, err error) error {
	if err == nil {
		return nil
	}

	// If it's a public error, preserve it to prevent double-wrapping.
	if _, ok := errors.AsType[*Error](err); ok {
		return err
	}

	// If it's a core error, map it
	if ce, ok := errors.AsType[*core.Error](err); ok {
		return &Error{
			Kind:     mapCoreKind(ce.Kind),
			Resource: mapCoreResource(ce.Resource),
			Op:       op,
			Details:  ce.Details,
			Err:      err,
		}
	}

	// Unknown or non-core error maps to an internal error.
	return &Error{
		Kind: KindInternal,
		Op:   op,
		Err:  err,
	}
}

func publicErrorLabel(kind Kind, resource Resource) string {
	if resource == ResourceUnknown {
		return string(kind)
	}
	return fmt.Sprintf("%s %s", resource, kind)
}
