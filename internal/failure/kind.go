// Package failure defines shared error kinds and resource labels used at
// spiderw layer boundaries.
package failure

// Kind identifies what kind of failure occurred.
type Kind string

// Kind constants classify failures at internal semantic boundaries.
const (
	// KindUnavailable indicates that a resource or subsystem could not be
	// reached or did not expose the expected API.
	KindUnavailable Kind = "unavailable"

	// KindInvalidState indicates that spiderw observed invalid state.
	KindInvalidState Kind = "invalid state"

	// KindInvalidArgument indicates that the caller supplied an invalid
	// argument.
	KindInvalidArgument Kind = "invalid argument"

	// KindOperationFailed indicates an internal operation failed but is not
	// part of the stable public kind set.
	KindOperationFailed Kind = "operation failed"

	// KindInternal indicates an uncategorized internal failure.
	KindInternal Kind = "internal error"
)

// Public returns the public API equivalent for k.
func Public(k Kind) Kind {
	switch k {
	case KindUnavailable,
		KindInvalidState,
		KindInvalidArgument,
		KindInternal:
		return k
	default:
		return KindInternal
	}
}

// Resource identifies the spiderw object or subsystem a failure applies to.
type Resource string

// Resource constants classify failures by target object/subsystem.
const (
	// ResourceUnknown indicates that no specific resource is known.
	ResourceUnknown Resource = ""

	// ResourceClient identifies client-level failures.
	ResourceClient Resource = "client"

	// ResourceDaemon identifies failures involving the iwd daemon object.
	ResourceDaemon Resource = "daemon"

	// ResourceAdapter identifies failures involving an iwd adapter object.
	ResourceAdapter Resource = "adapter"

	// ResourceDevice identifies failures involving an iwd device object.
	ResourceDevice Resource = "device"

	// ResourceBasicServiceSet identifies failures involving an iwd basic service
	// set (BSS) object.
	ResourceBasicServiceSet Resource = "basic service set"

	// ResourceStation identifies failures involving an iwd station object.
	ResourceStation Resource = "station"

	// ResourceNetwork identifies failures involving an iwd network object.
	ResourceNetwork Resource = "network"
)

// String returns the stable resource label.
func (r Resource) String() string {
	return string(r)
}
