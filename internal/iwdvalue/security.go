package iwdvalue

// SecurityType identifies the security/authentication type of an iwd network.
type SecurityType string

// SecurityType constants identify canonical iwd network "Type" strings.
// SecurityTypeUnknown is reserved for invalid or unrecognized values.
const (
	// SecurityTypeUnknown represents an invalid or unrecognized security type.
	SecurityTypeUnknown SecurityType = ""

	// SecurityTypeOpen is an open (unsecured) network.
	SecurityTypeOpen SecurityType = "open"

	// SecurityTypeWEP is a WEP network.
	SecurityTypeWEP SecurityType = "wep"

	// SecurityTypePSK is a pre-shared-key (WPA-Personal) network.
	SecurityTypePSK SecurityType = "psk"

	// SecurityType8021x is an 802.1x (EAP / WPA-Enterprise) network.
	SecurityType8021x SecurityType = "8021x"
)

// String returns the canonical iwd string for the security type.
func (s SecurityType) String() string {
	if ValidSecurityType(s) {
		return string(s)
	}
	return "unknown"
}

// ParseSecurityType converts a canonical iwd network type string to a
// SecurityType.
func ParseSecurityType(str string) (SecurityType, bool) {
	t := SecurityType(str)
	if !ValidSecurityType(t) {
		return SecurityTypeUnknown, false
	}
	return t, true
}

// ValidSecurityType reports whether t is a recognized iwd network type.
func ValidSecurityType(t SecurityType) bool {
	switch t {
	case SecurityTypeOpen, SecurityTypeWEP, SecurityTypePSK, SecurityType8021x:
		return true
	default:
		return false
	}
}
