package iwdvalue

// NetworkType identifies the type of an iwd network, as reported by the "Type"
// property of Network and KnownNetwork. Most values describe the security
// (open/wep/psk/8021x); "hotspot" describes a network kind rather than security.
type NetworkType string

// NetworkType constants identify canonical iwd network "Type" strings.
// NetworkTypeUnknown is reserved for invalid or unrecognized values.
const (
	// NetworkTypeUnknown represents an invalid or unrecognized network type.
	NetworkTypeUnknown NetworkType = ""

	// NetworkTypeOpen is an open (unsecured) network.
	NetworkTypeOpen NetworkType = "open"

	// NetworkTypeWEP is a WEP network.
	NetworkTypeWEP NetworkType = "wep"

	// NetworkTypePSK is a pre-shared-key (WPA-Personal) network.
	NetworkTypePSK NetworkType = "psk"

	// NetworkType8021x is an 802.1x (EAP / WPA-Enterprise) network.
	NetworkType8021x NetworkType = "8021x"

	// NetworkTypeHotspot is a hotspot network. iwd reports this value only for
	// the Type of a KnownNetwork (never for a scanned Network).
	NetworkTypeHotspot NetworkType = "hotspot"
)

// String returns the canonical iwd string for the network type.
func (s NetworkType) String() string {
	if ValidNetworkType(s) {
		return string(s)
	}
	return "unknown"
}

// ParseNetworkType converts a canonical iwd network type string to a
// NetworkType.
func ParseNetworkType(str string) (NetworkType, bool) {
	t := NetworkType(str)
	if !ValidNetworkType(t) {
		return NetworkTypeUnknown, false
	}
	return t, true
}

// ValidNetworkType reports whether t is a recognized iwd network type.
func ValidNetworkType(t NetworkType) bool {
	switch t {
	case NetworkTypeOpen, NetworkTypeWEP, NetworkTypePSK, NetworkType8021x, NetworkTypeHotspot:
		return true
	default:
		return false
	}
}
