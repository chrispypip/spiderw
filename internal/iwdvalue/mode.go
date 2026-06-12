// Package iwdvalue defines canonical iwd values shared across spiderw layers.
package iwdvalue

// Mode identifies an iwd operating mode.
type Mode string

// Mode constants identify canonical iwd mode strings.
// ModeUnknown is reserved for invalid or unrecognized values.
const (
	// ModeUnknown represents an invalid or unrecognized mode.
	ModeUnknown Mode = ""

	// ModeStation is the iwd station mode.
	ModeStation Mode = "station"

	// ModeAP is the iwd access point mode.
	ModeAP Mode = "ap"

	// ModeAdHoc is the iwd ad-hoc mode.
	ModeAdHoc Mode = "ad-hoc"
)

// String returns the canonical iwd string for the mode.
func (m Mode) String() string {
	if ValidMode(m) {
		return string(m)
	}
	return "unknown"
}

// ParseMode converts a canonical iwd mode string to an Mode.
func ParseMode(s string) (Mode, bool) {
	mode := Mode(s)
	if !ValidMode(mode) {
		return ModeUnknown, false
	}
	return mode, true
}

// ValidMode reports whether mode is a recognized iwd mode.
func ValidMode(mode Mode) bool {
	switch mode {
	case ModeStation, ModeAP, ModeAdHoc:
		return true
	default:
		return false
	}
}
