package spiderw

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
	"github.com/chrispypip/spiderw/internal/logging"
)

// StationState identifies an iwd station's connection state exposed by spiderw.
type StationState string

// StationState constants identify the station connection states.
// StationStateUnknown is reserved for invalid or unrecognized values.
const (
	// StationStateUnknown represents an invalid or unrecognized station state.
	StationStateUnknown StationState = StationState(iwdvalue.StationStateUnknown)

	// StationStateConnected means the station is connected to a network.
	StationStateConnected StationState = StationState(iwdvalue.StationStateConnected)

	// StationStateDisconnected means the station is not connected.
	StationStateDisconnected StationState = StationState(iwdvalue.StationStateDisconnected)

	// StationStateConnecting means the station is establishing a connection.
	StationStateConnecting StationState = StationState(iwdvalue.StationStateConnecting)

	// StationStateDisconnecting means the station is tearing down a connection.
	StationStateDisconnecting StationState = StationState(iwdvalue.StationStateDisconnecting)

	// StationStateRoaming means the station is roaming between access points.
	StationStateRoaming StationState = StationState(iwdvalue.StationStateRoaming)
)

// String returns the canonical iwd string for the station state.
func (s StationState) String() string {
	return iwdvalue.StationState(s).String()
}

// StationPropertiesChanged describes station properties reported by a D-Bus
// PropertiesChanged signal. Changed contains the new values by property name;
// Invalidated contains property names whose values should be re-read if needed.
type StationPropertiesChanged struct {
	// Changed contains new property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// StationProperties is a snapshot of all station properties read in a single
// D-Bus call. State and Scanning are always reported; the remaining fields are
// nil when absent.
type StationProperties struct {
	// State is the station's current connection state.
	State StationState

	// Scanning reports whether the station is currently scanning.
	Scanning bool

	// ConnectedNetwork is the object path of the network the station is connected
	// to, or nil when the station is not connected. Resolve it with Client.Network.
	ConnectedNetwork *string

	// ConnectedAccessPoint is the object path of the BSS the station is connected
	// to, or nil when disconnected or unreported. Resolve it with
	// Client.BasicServiceSet. iwd marks this property experimental.
	ConnectedAccessPoint *string

	// Affinities is the object paths of the BSSes the station has a roaming
	// affinity for, or nil when unreported. iwd marks this property experimental.
	Affinities []string
}

// Station provides high-level operations for a specific iwd station object.
//
// A station is a device operating in station (client) mode; it shares its object
// path with the Device. Station covers connection state and scanning; connecting
// to a network is done through Network.Connect.
type Station struct {
	core core.StationIface
	path string
}

func newStation(c core.StationIface, path string) *Station {
	if c == nil {
		return nil
	}
	return &Station{core: c, path: path}
}

// Path returns the D-Bus object path the station was constructed from.
//
// Path is static station identity, not an iwd property: it requires no D-Bus
// round-trip and never fails. Path returns "" for a nil receiver.
func (s *Station) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Station) coreStation(ctx context.Context, op string) (core.StationIface, error) {
	if s == nil || s.core == nil {
		logging.FromContext(ctx).Error(ctx, "station wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return s.core, nil
}

// State returns the station's current connection state.
func (s *Station) State(ctx context.Context) (StationState, error) {
	return delegate(ctx, "Station.State", s.coreStation, func(ctx context.Context, c core.StationIface) (StationState, error) {
		cs, err := c.State(ctx)
		if err != nil {
			return StationStateUnknown, err
		}
		return convertStationStateCoreToPublic(cs)
	})
}

// Scanning reports whether the station is currently scanning for networks.
func (s *Station) Scanning(ctx context.Context) (bool, error) {
	return delegate(ctx, "Station.Scanning", s.coreStation, func(ctx context.Context, c core.StationIface) (bool, error) {
		return c.Scanning(ctx)
	})
}

// ConnectedNetwork returns the object path of the network the station is
// connected to, or nil when the station is not connected.
//
// Resolve it to a handle with Client.Network.
func (s *Station) ConnectedNetwork(ctx context.Context) (*string, error) {
	return delegate(ctx, "Station.ConnectedNetwork", s.coreStation, func(ctx context.Context, c core.StationIface) (*string, error) {
		return c.ConnectedNetwork(ctx)
	})
}

// ConnectedAccessPoint returns the object path of the BSS the station is
// connected to, or nil when disconnected or unreported.
//
// Resolve it to a handle with Client.BasicServiceSet. iwd marks this property
// experimental.
func (s *Station) ConnectedAccessPoint(ctx context.Context) (*string, error) {
	return delegate(ctx, "Station.ConnectedAccessPoint", s.coreStation, func(ctx context.Context, c core.StationIface) (*string, error) {
		return c.ConnectedAccessPoint(ctx)
	})
}

// Affinities returns the object paths of the BSSes the station has a roaming
// affinity for, or nil when unreported. iwd marks this property experimental.
func (s *Station) Affinities(ctx context.Context) ([]string, error) {
	return delegate(ctx, "Station.Affinities", s.coreStation, func(ctx context.Context, c core.StationIface) ([]string, error) {
		return c.Affinities(ctx)
	})
}

// Properties reads every station property in a single D-Bus call
// (Properties.GetAll) instead of one call per property. Prefer it when you need
// several properties at once, such as building an overview of a station.
func (s *Station) Properties(ctx context.Context) (*StationProperties, error) {
	return delegate(ctx, "Station.Properties", s.coreStation, func(ctx context.Context, c core.StationIface) (*StationProperties, error) {
		cp, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}

		state, err := convertStationStateCoreToPublic(cp.State)
		if err != nil {
			return nil, err
		}

		return &StationProperties{
			State:                state,
			Scanning:             cp.Scanning,
			ConnectedNetwork:     cp.ConnectedNetwork,
			ConnectedAccessPoint: cp.ConnectedAccessPoint,
			Affinities:           cp.Affinities,
		}, nil
	})
}

// OrderedNetwork is one scanned network and its signal strength, as returned by
// OrderedNetworks.
type OrderedNetwork struct {
	// Network is the object path of the network. Resolve it with Client.Network.
	Network string

	// SignalStrength is the signal strength in dBm (e.g. -60.5). iwd reports it in
	// units of 100 * dBm; spiderw exposes it as dBm here.
	SignalStrength float64
}

// Scan schedules a network scan on the station. It is asynchronous: the call
// returns once the scan is scheduled, and the station's Scanning property tracks
// progress. Subscribe with SubscribeScanningChanged to observe completion, then
// read results with OrderedNetworks.
func (s *Station) Scan(ctx context.Context) error {
	return do(ctx, "Station.Scan", s.coreStation, func(ctx context.Context, c core.StationIface) error {
		return c.Scan(ctx)
	})
}

// OrderedNetworks returns the networks from the most recent scan, ordered by iwd
// with the strongest signal first. No scan is required to read the last results.
func (s *Station) OrderedNetworks(ctx context.Context) ([]OrderedNetwork, error) {
	return delegate(ctx, "Station.OrderedNetworks", s.coreStation, func(ctx context.Context, c core.StationIface) ([]OrderedNetwork, error) {
		raw, err := c.OrderedNetworks(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]OrderedNetwork, 0, len(raw))
		for _, n := range raw {
			out = append(out, OrderedNetwork{
				Network:        n.Network,
				SignalStrength: float64(n.SignalStrength) / 100,
			})
		}
		return out, nil
	})
}

// SetAffinities sets the BSS object paths the station should stay affine to (an
// experimental iwd property). Each path must be a non-empty absolute object path,
// and should be a BSS of the currently connected network (see
// Network.ExtendedServiceSet).
//
// Affinities depends on driver support: on hardware that cannot honor it, iwd
// rejects the write and the returned error matches ErrNotSupported via
// errors.Is.
func (s *Station) SetAffinities(ctx context.Context, paths []string) error {
	return do(ctx, "Station.SetAffinities", s.coreStation, func(ctx context.Context, c core.StationIface) error {
		return c.SetAffinities(ctx, slices.Clone(paths))
	})
}

// HiddenAccessPoint is one hidden access point found in the last scan, as
// returned by HiddenAccessPoints.
type HiddenAccessPoint struct {
	// Address is the BSS hardware (BSSID) address.
	Address string

	// SignalStrength is the signal strength in dBm (e.g. -60.5). iwd reports it in
	// units of 100 * dBm; spiderw exposes it as dBm here.
	SignalStrength float64

	// Type is the network security type.
	Type NetworkType
}

// Disconnect disconnects the station from its current network.
func (s *Station) Disconnect(ctx context.Context) error {
	return do(ctx, "Station.Disconnect", s.coreStation, func(ctx context.Context, c core.StationIface) error {
		return c.Disconnect(ctx)
	})
}

// ConnectHiddenNetwork connects to a hidden network by SSID. A secured hidden
// network requires a registered credentials agent (register one with
// Client.RegisterAgent before calling); without one, iwd surfaces an error
// matching ErrNoAgent.
func (s *Station) ConnectHiddenNetwork(ctx context.Context, name string) error {
	return do(ctx, "Station.ConnectHiddenNetwork", s.coreStation, func(ctx context.Context, c core.StationIface) error {
		return c.ConnectHiddenNetwork(ctx, name)
	})
}

// HiddenAccessPoints returns the hidden access points found in the most recent
// scan. It is an experimental iwd operation: hardware that cannot provide it
// makes iwd reject the call, and the returned error matches ErrNotSupported via
// errors.Is.
func (s *Station) HiddenAccessPoints(ctx context.Context) ([]HiddenAccessPoint, error) {
	return delegate(ctx, "Station.HiddenAccessPoints", s.coreStation, func(ctx context.Context, c core.StationIface) ([]HiddenAccessPoint, error) {
		raw, err := c.HiddenAccessPoints(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]HiddenAccessPoint, 0, len(raw))
		for _, ap := range raw {
			netType, err := convertNetworkType(ap.Type)
			if err != nil {
				return nil, err
			}
			out = append(out, HiddenAccessPoint{
				Address:        ap.Address,
				SignalStrength: float64(ap.SignalStrength) / 100,
				Type:           netType,
			})
		}
		return out, nil
	})
}

// SubscribePropertiesChanged registers fn for station property-change signals and
// returns a handle that unregisters the callback.
func (s *Station) SubscribePropertiesChanged(ctx context.Context, fn func(StationPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Station.SubscribePropertiesChanged"

	coreStation, err := s.coreStation(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceStation, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreStation.SubscribePropertiesChanged(ctx, func(core core.StationPropertiesChanged) {
		changed := make(map[string]any, len(core.Changed))
		maps.Copy(changed, core.Changed)

		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if core.Invalidated != nil {
			invalidated = slices.Clone(core.Invalidated)
		}

		fn(StationPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeStateChanged registers fn for station connection-state changes and
// returns a handle that unregisters the callback.
func (s *Station) SubscribeStateChanged(ctx context.Context, fn func(StationState)) (UnsubscribeFunc, error) {
	const op = "Station.SubscribeStateChanged"

	coreStation, err := s.coreStation(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceStation, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreStation.SubscribeStateChanged(ctx, func(cs core.StationState) {
		// Lower layers only deliver recognized states; drop anything else rather
		// than surfacing StationStateUnknown to the caller.
		state, err := convertStationStateCoreToPublic(cs)
		if err != nil {
			return
		}
		fn(state)
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeScanningChanged registers fn for station scanning-state changes and
// returns a handle that unregisters the callback.
func (s *Station) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Station.SubscribeScanningChanged"

	coreStation, err := s.coreStation(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceStation, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreStation.SubscribeScanningChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

func convertStationStateCoreToPublic(state core.StationState) (StationState, error) {
	if !iwdvalue.ValidStationState(state) {
		details := fmt.Sprintf("invalid station state %q", state)
		return StationStateUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceStation, Op: "Station.convertState", Details: details, Err: ErrInvalidArgument}
	}
	return StationState(state), nil
}
