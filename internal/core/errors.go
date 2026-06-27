package core

import (
	"errors"
	"fmt"

	"github.com/chrispypip/spiderw/internal/failure"
	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// -----------------------------------------------------------------------------
// Error kinds
// -----------------------------------------------------------------------------
//
// These represent semantic, high-level categories of failure that callers
// will care about. They are intentionally stable.
//
// Example kinds:
//   - "unavailable"
//   - "invalid state"
//   - "operation failed"
//
// These are used instead of raw D-Bus error names, because spiderw's semantics
// are defined here.
//

// Kind identifies a stable internal core error category.
type Kind = failure.Kind

// Kind constants classify core-layer failures.
const (
	// KindUnavailable indicates that a resource or subsystem could not be
	// reached or did not expose the expected API.
	KindUnavailable = failure.KindUnavailable

	// KindInvalidState indicates that core observed invalid state.
	KindInvalidState = failure.KindInvalidState

	// KindInvalidArgument indicates that a caller supplied an invalid argument.
	KindInvalidArgument = failure.KindInvalidArgument

	// KindOperationFailed indicates an internal operation failed but should map
	// to a public internal error.
	KindOperationFailed = failure.KindOperationFailed
)

// Resource identifies which core resource a failure applies to.
type Resource = failure.Resource

// Resource constants classify failures by target object/subsystem.
const (
	// ResourceUnknown indicates that no specific resource is known.
	ResourceUnknown = failure.ResourceUnknown

	// ResourceClient identifies client-level failures.
	ResourceClient = failure.ResourceClient

	// ResourceDaemon identifies failures involving the iwd daemon object.
	ResourceDaemon = failure.ResourceDaemon

	// ResourceAdapter identifies failures involving an iwd adapter object.
	ResourceAdapter = failure.ResourceAdapter

	// ResourceDevice identifies failures involving an iwd device object.
	ResourceDevice = failure.ResourceDevice

	// ResourceBasicServiceSet identifies failures involving an iwd basic service
	// set (BSS) object.
	ResourceBasicServiceSet = failure.ResourceBasicServiceSet

	// ResourceStation identifies failures involving an iwd station object.
	ResourceStation = failure.ResourceStation

	// ResourceNetwork identifies failures involving an iwd network object.
	ResourceNetwork = failure.ResourceNetwork

	// ResourceKnownNetwork identifies failures involving an iwd known-network
	// object.
	ResourceKnownNetwork = failure.ResourceKnownNetwork

	// ResourceAgent identifies failures involving the iwd credentials agent or
	// agent manager.
	ResourceAgent = failure.ResourceAgent
)

// Error sentinels support errors.Is checks in core-layer errors.
var (
	// ErrCore marks structured errors produced by the core layer.
	ErrCore = errors.New("core error")

	// ErrDaemonNotInitialized indicates that a Daemon wrapper has no raw
	// backend.
	ErrDaemonNotInitialized = errors.New("daemon not initialized")

	// ErrAdapterNotInitialized indicates that an Adapter wrapper has no raw
	// backend.
	ErrAdapterNotInitialized = errors.New("adapter not initialized")

	// ErrDeviceNotInitialized indicates that a Device wrapper has no raw
	// backend.
	ErrDeviceNotInitialized = errors.New("device not initialized")

	// ErrBasicServiceSetNotInitialized indicates that a BasicServiceSet wrapper
	// has no raw backend.
	ErrBasicServiceSetNotInitialized = errors.New("basic service set not initialized")

	// ErrNetworkNotInitialized indicates that a Network wrapper has no raw
	// backend.
	ErrNetworkNotInitialized = errors.New("network not initialized")

	// ErrKnownNetworkNotInitialized indicates that a KnownNetwork wrapper has no
	// raw backend.
	ErrKnownNetworkNotInitialized = errors.New("known network not initialized")

	// ErrAgentNotInitialized indicates that an Agent wrapper has no backend.
	ErrAgentNotInitialized = errors.New("agent not initialized")

	// ErrNoAgent indicates that iwd rejected an operation because no credentials
	// agent is registered. It is re-exported from the iwdbus layer so callers can
	// match it with errors.Is through the core and public error chains.
	ErrNoAgent = iwdbus.ErrNoAgent

	// The following sentinels mirror named iwd D-Bus errors (e.g. from
	// Network.Connect), re-exported so callers can match them with errors.Is.
	ErrAborted       = iwdbus.ErrAborted
	ErrBusy          = iwdbus.ErrBusy
	ErrFailed        = iwdbus.ErrFailed
	ErrNotSupported  = iwdbus.ErrNotSupported
	ErrTimeout       = iwdbus.ErrTimeout
	ErrInProgress    = iwdbus.ErrInProgress
	ErrNotConfigured = iwdbus.ErrNotConfigured

	ErrNotFound           = iwdbus.ErrNotFound
	ErrAlreadyExists      = iwdbus.ErrAlreadyExists
	ErrInvalidArguments   = iwdbus.ErrInvalidArguments
	ErrInvalidFormat      = iwdbus.ErrInvalidFormat
	ErrNotConnected       = iwdbus.ErrNotConnected
	ErrNotImplemented     = iwdbus.ErrNotImplemented
	ErrServiceSetOverlap  = iwdbus.ErrServiceSetOverlap
	ErrAlreadyProvisioned = iwdbus.ErrAlreadyProvisioned
	ErrNotHidden          = iwdbus.ErrNotHidden
	ErrNotAvailable       = iwdbus.ErrNotAvailable
)

func newError(kind Kind, resource Resource, op, details string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Kind:     kind,
		Resource: resource,
		Op:       op,
		Details:  details,
		Err:      err,
	}
}

// -----------------------------------------------------------------------------
// Structured core error type
// -----------------------------------------------------------------------------
//
// This is the main error type used inside internal/core.
//
// It wraps:
//   - Op: the semantic operation ("Scan", "Connect", "InitAdapter")
//   - Kind: the category of failure (see above)
//   - Details: optional human-friendly explanatory text
//   - Err: the wrapped underlying error (D-Bus or other)
//
// This allows excellent traceability while still giving callers a clean,
// semantic, machine-readable error surface.
//

// Error is the structured error type used inside internal/core.
type Error struct {
	Kind     Kind     // category of failure
	Resource Resource // resource or subsystem involved, when known
	Op       string   // semantic operation
	Details  string   // optional additional info
	Err      error    // wrapped underlying error
}

// Error returns a human-readable core error string.
func (e *Error) Error() string {
	label := errorLabel(e.Kind, e.Resource)
	if e.Details != "" {
		return fmt.Sprintf("%s: Op=%s: %v (%s)", label, e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("%s: Op=%s: %v", label, e.Op, e.Err)
}

// Unwrap returns the sentinel and underlying errors for errors.Is, errors.As,
// and errors.AsType.
func (e *Error) Unwrap() error {
	return errors.Join(ErrCore, e.Err)
}

type unavailablePolicy struct {
	resource            Resource
	includeDBusProperty bool
}

var (
	daemonUnavailablePolicy       = unavailablePolicy{resource: ResourceDaemon}
	adapterUnavailablePolicy      = unavailablePolicy{resource: ResourceAdapter, includeDBusProperty: true}
	deviceUnavailablePolicy       = unavailablePolicy{resource: ResourceDevice, includeDBusProperty: true}
	bssUnavailablePolicy          = unavailablePolicy{resource: ResourceBasicServiceSet, includeDBusProperty: true}
	networkUnavailablePolicy      = unavailablePolicy{resource: ResourceNetwork, includeDBusProperty: true}
	knownNetworkUnavailablePolicy = unavailablePolicy{resource: ResourceKnownNetwork, includeDBusProperty: true}
	// Agent operations are method calls (Register/Unregister) and object
	// export/unexport, never property reads, so includeDBusProperty stays false.
	agentUnavailablePolicy = unavailablePolicy{resource: ResourceAgent}
)

func wrapUnavailable(op, details string, err error, policy unavailablePolicy) error {
	if err == nil {
		return nil
	}
	if isUnavailableDBusError(err, policy.includeDBusProperty) {
		return newError(KindUnavailable, policy.resource, op, details, err)
	}
	return newError(KindOperationFailed, policy.resource, op, details, err)
}

func isUnavailableDBusError(err error, includeProperty bool) bool {
	if errors.Is(err, iwdbus.ErrDBusConnection) ||
		errors.Is(err, iwdbus.ErrDBusMethod) ||
		errors.Is(err, iwdbus.ErrDBusIntrospection) ||
		errors.Is(err, iwdbus.ErrDBusVariant) {
		return true
	}
	return includeProperty && errors.Is(err, iwdbus.ErrDBusProperty)
}

// WrapDaemonUnavailable classifies D-Bus daemon failures by kind and resource.
func WrapDaemonUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, daemonUnavailablePolicy)
}

// WrapAdapterUnavailable classifies D-Bus adapter failures by kind and resource.
func WrapAdapterUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, adapterUnavailablePolicy)
}

// WrapDeviceUnavailable classifies D-Bus device failures by kind and resource.
func WrapDeviceUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, deviceUnavailablePolicy)
}

// WrapBasicServiceSetUnavailable classifies D-Bus basic-service-set failures by
// kind and resource.
func WrapBasicServiceSetUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, bssUnavailablePolicy)
}

// WrapNetworkUnavailable classifies D-Bus network failures by kind and resource.
func WrapNetworkUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, networkUnavailablePolicy)
}

// WrapKnownNetworkUnavailable classifies D-Bus known-network failures by kind and
// resource.
func WrapKnownNetworkUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, knownNetworkUnavailablePolicy)
}

// WrapAgentUnavailable classifies D-Bus agent/agent-manager failures by kind and
// resource.
func WrapAgentUnavailable(op, details string, err error) error {
	return wrapUnavailable(op, details, err, agentUnavailablePolicy)
}

// WrapInvalidState wraps invalid state issues for resource.
func WrapInvalidState(resource Resource, op, details string, err error) error {
	return newError(KindInvalidState, resource, op, details, err)
}

// WrapInvalidArgument wraps invalid argument issues for resource.
func WrapInvalidArgument(resource Resource, op, details string, err error) error {
	return newError(KindInvalidArgument, resource, op, details, err)
}

// WrapOperationFailed wraps generic non-D-Bus failures for resource.
func WrapOperationFailed(resource Resource, op, details string, err error) error {
	return newError(KindOperationFailed, resource, op, details, err)
}

func errorLabel(kind Kind, resource Resource) string {
	if resource == ResourceUnknown {
		return string(kind)
	}
	return fmt.Sprintf("%s %s", resource, kind)
}
