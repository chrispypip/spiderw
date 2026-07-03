package iwdvalue

// StationState identifies the connection state of an iwd station, as reported by
// the "State" property of net.connman.iwd.Station.
type StationState string

// StationState constants identify canonical iwd station "State" strings.
// StationStateUnknown is reserved for invalid or unrecognized values.
const (
	// StationStateUnknown represents an invalid or unrecognized station state.
	StationStateUnknown StationState = ""

	// StationStateConnected means the station is connected to a network.
	StationStateConnected StationState = "connected"

	// StationStateDisconnected means the station is not connected.
	StationStateDisconnected StationState = "disconnected"

	// StationStateConnecting means the station is establishing a connection.
	StationStateConnecting StationState = "connecting"

	// StationStateDisconnecting means the station is tearing down a connection.
	StationStateDisconnecting StationState = "disconnecting"

	// StationStateRoaming means the station is roaming between access points.
	StationStateRoaming StationState = "roaming"
)

// String returns the canonical iwd string for the station state.
func (s StationState) String() string {
	if ValidStationState(s) {
		return string(s)
	}
	return "unknown"
}

// ParseStationState converts a canonical iwd station state string to a
// StationState.
func ParseStationState(str string) (StationState, bool) {
	s := StationState(str)
	if !ValidStationState(s) {
		return StationStateUnknown, false
	}
	return s, true
}

// ValidStationState reports whether s is a recognized iwd station state.
func ValidStationState(s StationState) bool {
	switch s {
	case StationStateConnected, StationStateDisconnected, StationStateConnecting,
		StationStateDisconnecting, StationStateRoaming:
		return true
	default:
		return false
	}
}
