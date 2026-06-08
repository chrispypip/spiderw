// Package iwdvalue defines canonical iwd values shared across spiderw layers.
package iwdvalue

// AdapterMode identifies an iwd adapter operating mode.
type AdapterMode string

// Adapter mode constants identify canonical iwd adapter mode strings.
// AdapterModeUnknown is reserved for invalid or unrecognized values.
const (
	// AdapterModeUnknown represents an invalid or unrecognized adapter mode.
	AdapterModeUnknown AdapterMode = ""

	// AdapterModeStation is the iwd station adapter mode.
	AdapterModeStation AdapterMode = "station"

	// AdapterModeAP is the iwd access point adapter mode.
	AdapterModeAP AdapterMode = "ap"

	// AdapterModeAdHoc is the iwd ad-hoc adapter mode.
	AdapterModeAdHoc AdapterMode = "ad-hoc"
)

// String returns the canonical iwd string for the adapter mode.
func (m AdapterMode) String() string {
	if ValidAdapterMode(m) {
		return string(m)
	}
	return "unknown"
}

// ParseAdapterMode converts a canonical iwd mode string to an AdapterMode.
func ParseAdapterMode(s string) (AdapterMode, bool) {
	mode := AdapterMode(s)
	if !ValidAdapterMode(mode) {
		return AdapterModeUnknown, false
	}
	return mode, true
}

// ValidAdapterMode reports whether mode is a recognized iwd adapter mode.
func ValidAdapterMode(mode AdapterMode) bool {
	switch mode {
	case AdapterModeStation, AdapterModeAP, AdapterModeAdHoc:
		return true
	default:
		return false
	}
}
