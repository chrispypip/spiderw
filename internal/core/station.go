package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// StationState identifies an iwd station connection state.
type StationState = iwdvalue.StationState

// StationState constants identify canonical iwd station states.
// StationStateUnknown is reserved for invalid or unrecognized values.
const (
	// StationStateUnknown represents an invalid or unrecognized station state.
	StationStateUnknown = iwdvalue.StationStateUnknown

	// StationStateConnected means the station is connected to a network.
	StationStateConnected = iwdvalue.StationStateConnected

	// StationStateDisconnected means the station is not connected.
	StationStateDisconnected = iwdvalue.StationStateDisconnected

	// StationStateConnecting means the station is establishing a connection.
	StationStateConnecting = iwdvalue.StationStateConnecting

	// StationStateDisconnecting means the station is tearing down a connection.
	StationStateDisconnecting = iwdvalue.StationStateDisconnecting

	// StationStateRoaming means the station is roaming between access points.
	StationStateRoaming = iwdvalue.StationStateRoaming
)

// StationPropertiesChanged describes normalized station property-change data.
type StationPropertiesChanged struct {
	// Changed contains normalized property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

type stationRaw interface {
	GetState(ctx context.Context) (iwdbus.StationState, error)
	GetScanning(ctx context.Context) (bool, error)
	GetConnectedNetwork(ctx context.Context) (*string, error)
	GetConnectedAccessPoint(ctx context.Context) (*string, error)
	GetAffinities(ctx context.Context) ([]string, error)
	GetProperties(ctx context.Context) (*iwdbus.StationProperties, error)
	Scan(ctx context.Context) error
	GetOrderedNetworks(ctx context.Context) ([]iwdbus.OrderedNetwork, error)
	SetAffinities(ctx context.Context, paths []string) error
	SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.StationPropertiesChanged)) (iwdbus.UnsubscribeFunc, error)
	SubscribeStateChanged(ctx context.Context, fn func(iwdbus.StationState)) (iwdbus.UnsubscribeFunc, error)
	SubscribeScanningChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
}

// StationIface defines the core station operations used by the public layer.
type StationIface interface {
	State(ctx context.Context) (StationState, error)
	Scanning(ctx context.Context) (bool, error)
	ConnectedNetwork(ctx context.Context) (*string, error)
	ConnectedAccessPoint(ctx context.Context) (*string, error)
	Affinities(ctx context.Context) ([]string, error)
	Properties(ctx context.Context) (*StationProperties, error)
	Scan(ctx context.Context) error
	OrderedNetworks(ctx context.Context) ([]OrderedNetwork, error)
	SetAffinities(ctx context.Context, paths []string) error
	SubscribePropertiesChanged(ctx context.Context, fn func(StationPropertiesChanged)) (UnsubscribeFunc, error)
	SubscribeStateChanged(ctx context.Context, fn func(StationState)) (UnsubscribeFunc, error)
	SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
}

// StationProperties holds normalized station properties read in a single backend
// call. State and Scanning are always reported; the remaining fields are nil when
// absent (ConnectedNetwork/ConnectedAccessPoint when disconnected; Affinities and
// ConnectedAccessPoint are experimental and may be absent entirely).
type StationProperties struct {
	State                StationState
	Scanning             bool
	ConnectedNetwork     *string
	ConnectedAccessPoint *string
	Affinities           []string
}

// Station is the core-layer facade over a raw iwd station backend.
type Station struct {
	raw stationRaw
}

// NewStation wraps a raw station backend in a core-layer Station.
func NewStation(raw stationRaw) *Station {
	if raw == nil {
		return nil
	}
	return &Station{raw: raw}
}

func (s *Station) rawStation(op string) (stationRaw, error) {
	if s == nil || s.raw == nil {
		return nil, WrapInvalidState(ResourceStation, op, "station wrapper was nil", ErrStationNotInitialized)
	}
	return s.raw, nil
}

// State returns the normalized station connection state.
func (s *Station) State(ctx context.Context) (StationState, error) {
	const op = "Station.State"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return StationStateUnknown, err
	}

	raw, err := rawStation.GetState(ctx)
	if err != nil {
		return StationStateUnknown, WrapStationUnavailable(op, "failed querying iwd Station state", err)
	}

	return validateStationState(op, raw)
}

// Scanning reports whether the station is currently scanning.
func (s *Station) Scanning(ctx context.Context) (bool, error) {
	const op = "Station.Scanning"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return false, err
	}

	value, err := rawStation.GetScanning(ctx)
	if err != nil {
		return false, WrapStationUnavailable(op, "failed querying iwd Station scanning", err)
	}

	return value, nil
}

// ConnectedNetwork returns the object path of the network the station is
// connected to, or nil when the station is not connected.
func (s *Station) ConnectedNetwork(ctx context.Context) (*string, error) {
	const op = "Station.ConnectedNetwork"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawStation.GetConnectedNetwork(ctx)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed querying iwd Station connected network", err)
	}

	return normalizeOptionalString(raw), nil
}

// ConnectedAccessPoint returns the object path of the BSS the station is
// connected to, or nil when it is not connected or iwd does not report it.
func (s *Station) ConnectedAccessPoint(ctx context.Context) (*string, error) {
	const op = "Station.ConnectedAccessPoint"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawStation.GetConnectedAccessPoint(ctx)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed querying iwd Station connected access point", err)
	}

	return normalizeOptionalString(raw), nil
}

// Affinities returns the object paths of the BSSes the station has a roaming
// affinity for, or nil when iwd does not report the property.
func (s *Station) Affinities(ctx context.Context) ([]string, error) {
	const op = "Station.Affinities"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawStation.GetAffinities(ctx)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed querying iwd Station affinities", err)
	}

	return raw, nil
}

// Properties returns all normalized station properties read in a single backend
// call (Properties.GetAll), applying the same normalization as the per-property
// getters: State is validated, and ConnectedNetwork is nil when absent.
func (s *Station) Properties(ctx context.Context) (*StationProperties, error) {
	const op = "Station.Properties"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawStation.GetProperties(ctx)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed querying iwd Station properties", err)
	}

	state, err := validateStationState(op, raw.State)
	if err != nil {
		return nil, err
	}

	return &StationProperties{
		State:                state,
		Scanning:             raw.Scanning,
		ConnectedNetwork:     normalizeOptionalString(raw.ConnectedNetwork),
		ConnectedAccessPoint: normalizeOptionalString(raw.ConnectedAccessPoint),
		Affinities:           raw.Affinities,
	}, nil
}

// OrderedNetwork is one scanned network and its signal strength, as returned by
// OrderedNetworks.
type OrderedNetwork struct {
	// Network is the object path of the network.
	Network string

	// SignalStrength is the signal strength in units of 100 * dBm (iwd's native
	// unit), e.g. -6000 means -60 dBm.
	SignalStrength int16
}

// Scan schedules a network scan. It is asynchronous; the station's Scanning
// property tracks progress (subscribe to observe it return to false).
func (s *Station) Scan(ctx context.Context) error {
	const op = "Station.Scan"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return err
	}

	if err := rawStation.Scan(ctx); err != nil {
		return WrapStationUnavailable(op, "failed scheduling iwd Station scan", err)
	}
	return nil
}

// OrderedNetworks returns the networks from the most recent scan, ordered by iwd
// (best signal first). Each network path is validated to be absolute.
func (s *Station) OrderedNetworks(ctx context.Context) ([]OrderedNetwork, error) {
	const op = "Station.OrderedNetworks"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawStation.GetOrderedNetworks(ctx)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed querying iwd Station ordered networks", err)
	}

	out := make([]OrderedNetwork, 0, len(raw))
	for _, n := range raw {
		path := strings.TrimSpace(string(n.Network))
		if path == "" || !strings.HasPrefix(path, "/") {
			return nil, WrapInvalidState(ResourceStation, op, "station returned invalid network path", fmt.Errorf("invalid network path %q", n.Network))
		}
		out = append(out, OrderedNetwork{Network: path, SignalStrength: n.SignalStrength})
	}
	return out, nil
}

// SetAffinities sets the BSS object paths the station should stay affine to.
// Each path must be a non-empty absolute object path. Affinities is an
// experimental, driver-dependent property; hardware that cannot honor it makes
// iwd reject the write, surfaced as a matchable ErrNotSupported.
func (s *Station) SetAffinities(ctx context.Context, paths []string) error {
	const op = "Station.SetAffinities"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return err
	}

	for _, p := range paths {
		if strings.TrimSpace(p) == "" || !strings.HasPrefix(p, "/") {
			return WrapInvalidArgument(ResourceStation, op, "affinity path must be a non-empty absolute object path", fmt.Errorf("invalid affinity path %q", p))
		}
	}

	if err := rawStation.SetAffinities(ctx, paths); err != nil {
		return WrapStationUnavailable(op, "failed setting iwd Station affinities", err)
	}
	return nil
}

// SubscribePropertiesChanged registers fn for normalized property-change events.
func (s *Station) SubscribePropertiesChanged(ctx context.Context, fn func(StationPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Station.SubscribePropertiesChanged"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceStation, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawStation.SubscribePropertiesChanged(ctx, func(raw iwdbus.StationPropertiesChanged) {
		changed := make(map[string]any, len(raw.Changed))
		for k, v := range raw.Changed {
			changed[k] = v.Value()
		}
		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if raw.Invalidated != nil {
			invalidated = slices.Clone(raw.Invalidated)
		}

		fn(StationPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed to call iwd Station subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeStateChanged registers fn for normalized connection-state events.
func (s *Station) SubscribeStateChanged(ctx context.Context, fn func(StationState)) (UnsubscribeFunc, error) {
	const op = "Station.SubscribeStateChanged"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceStation, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawStation.SubscribeStateChanged(ctx, fn)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed to call iwd Station subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeScanningChanged registers fn for normalized scanning-state events.
func (s *Station) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Station.SubscribeScanningChanged"

	rawStation, err := s.rawStation(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceStation, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawStation.SubscribeScanningChanged(ctx, fn)
	if err != nil {
		return nil, WrapStationUnavailable(op, "failed to call iwd Station subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// ParseStationState converts a canonical iwd station state string to a
// StationState.
func ParseStationState(s string) (StationState, error) {
	state, ok := iwdvalue.ParseStationState(s)
	if !ok {
		details := fmt.Sprintf("invalid station state %q", s)
		return StationStateUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceStation, Op: "Station.ParseStationState", Details: details, Err: ErrCore}
	}
	return state, nil
}

// validateStationState ensures the backend reported a recognized iwd station
// state, treating an unknown value as invalid state rather than silently
// propagating it.
func validateStationState(op string, state iwdbus.StationState) (StationState, error) {
	if !iwdvalue.ValidStationState(state) {
		details := fmt.Sprintf("station reported unknown state %q", state)
		return StationStateUnknown, WrapInvalidState(ResourceStation, op, details, fmt.Errorf("missing or invalid State field"))
	}
	return state, nil
}
