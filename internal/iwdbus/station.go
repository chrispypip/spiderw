package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// IwdStationIface is the fully qualified D-Bus interface name for iwd stations.
// The Station interface is exported on a device object when the device is in
// station mode, so a Station shares its object path with the Device.
const IwdStationIface = IwdService + ".Station"

// StationState identifies a raw iwd station connection state.
type StationState = iwdvalue.StationState

// StationState constants identify raw iwd station states.
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

// StationPropertiesChanged describes raw D-Bus station property-change data.
type StationPropertiesChanged struct {
	// Changed contains raw D-Bus variants keyed by property name.
	Changed map[string]dbus.Variant

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// Station wraps an iwd Station object using runtime introspection.
type Station struct {
	call    caller
	signals signalSource
}

// NewStation creates a Station for the given iwd object path (a device path).
func NewStation(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*Station, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdStationIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdStationIface)
	}
	return &Station{
		call:    caller(intro),
		signals: signalSource(intro),
	}, nil
}

// GetState reads and parses the State property.
func (s *Station) GetState(ctx context.Context) (StationState, error) {
	if err := s.ensureInitialized(); err != nil {
		return StationStateUnknown, WrapConnection("Station.ensureInitialized", err)
	}

	value, err := s.call.GetProperty(ctx, IwdStationIface, "State")
	if err != nil {
		return StationStateUnknown, WrapProperty(IwdStationIface, "State", err)
	}
	return parseStationState(value)
}

// GetScanning reads the Scanning property.
func (s *Station) GetScanning(ctx context.Context) (bool, error) {
	if err := s.ensureInitialized(); err != nil {
		return false, WrapConnection("Station.ensureInitialized", err)
	}

	value, err := s.call.GetProperty(ctx, IwdStationIface, "Scanning")
	if err != nil {
		return false, WrapProperty(IwdStationIface, "Scanning", err)
	}

	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Scanning", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// GetConnectedNetwork reads the ConnectedNetwork property, the object path of the
// network the station is connected to. It is optional: iwd omits it when the
// station is not connected, in which case a nil pointer (not an error) is
// returned.
func (s *Station) GetConnectedNetwork(ctx context.Context) (*string, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, WrapConnection("Station.ensureInitialized", err)
	}

	value, err := s.call.GetProperty(ctx, IwdStationIface, "ConnectedNetwork")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdStationIface, "ConnectedNetwork", err)
	}
	return parseStationObjectPath("ConnectedNetwork", value)
}

// GetConnectedAccessPoint reads the ConnectedAccessPoint property, the object
// path of the BasicServiceSet (BSS) the station is connected to. It is optional
// (iwd marks it experimental and omits it when disconnected or unsupported), so a
// nil pointer (not an error) is returned when absent.
func (s *Station) GetConnectedAccessPoint(ctx context.Context) (*string, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, WrapConnection("Station.ensureInitialized", err)
	}

	value, err := s.call.GetProperty(ctx, IwdStationIface, "ConnectedAccessPoint")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdStationIface, "ConnectedAccessPoint", err)
	}
	return parseStationObjectPath("ConnectedAccessPoint", value)
}

// GetAffinities reads the Affinities property, the object paths of the BSSes the
// station has a roaming affinity for. It is optional (iwd marks it experimental
// and omits it when unsupported), so nil (not an error) is returned when absent.
func (s *Station) GetAffinities(ctx context.Context) ([]string, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, WrapConnection("Station.ensureInitialized", err)
	}

	value, err := s.call.GetProperty(ctx, IwdStationIface, "Affinities")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdStationIface, "Affinities", err)
	}
	return parseStationAffinities(value)
}

// StationProperties holds every station property read in a single
// Properties.GetAll call. State and Scanning are always reported; the remaining
// fields are optional and left nil when absent (ConnectedNetwork and
// ConnectedAccessPoint when the station is not connected; Affinities and
// ConnectedAccessPoint are also experimental and may be absent entirely).
type StationProperties struct {
	State                StationState
	Scanning             bool
	ConnectedNetwork     *string
	ConnectedAccessPoint *string
	Affinities           []string
}

// GetProperties reads every station property in a single Properties.GetAll call
// instead of one Get per property. State and Scanning are required; a missing one
// is an error. ConnectedNetwork is optional and left nil when absent.
func (s *Station) GetProperties(ctx context.Context) (*StationProperties, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, WrapConnection("Station.ensureInitialized", err)
	}

	raw, err := s.call.GetAll(ctx, IwdStationIface)
	if err != nil {
		return nil, WrapProperty(IwdStationIface, "GetAll", err)
	}

	props := &StationProperties{}

	stateV, ok := raw["State"]
	if !ok {
		return nil, WrapProperty(IwdStationIface, "State", fmt.Errorf("missing required property"))
	}
	state, err := parseStationState(stateV.Value())
	if err != nil {
		return nil, err
	}
	props.State = state

	scanningV, ok := raw["Scanning"]
	if !ok {
		return nil, WrapProperty(IwdStationIface, "Scanning", fmt.Errorf("missing required property"))
	}
	scanning, ok := scanningV.Value().(bool)
	if !ok {
		return nil, WrapVariant("Scanning", fmt.Errorf("expected bool, got %T", scanningV.Value()))
	}
	props.Scanning = scanning

	if connectedV, ok := raw["ConnectedNetwork"]; ok {
		connected, err := parseStationObjectPath("ConnectedNetwork", connectedV.Value())
		if err != nil {
			return nil, err
		}
		props.ConnectedNetwork = connected
	}

	if apV, ok := raw["ConnectedAccessPoint"]; ok {
		ap, err := parseStationObjectPath("ConnectedAccessPoint", apV.Value())
		if err != nil {
			return nil, err
		}
		props.ConnectedAccessPoint = ap
	}

	if affV, ok := raw["Affinities"]; ok {
		affinities, err := parseStationAffinities(affV.Value())
		if err != nil {
			return nil, err
		}
		props.Affinities = affinities
	}

	return props, nil
}

// OrderedNetwork is one entry of GetOrderedNetworks: a scanned network and its
// signal strength.
type OrderedNetwork struct {
	// Network is the D-Bus object path of the network.
	Network dbus.ObjectPath

	// SignalStrength is the signal strength in units of 100 * dBm (iwd's native
	// unit), e.g. -6000 means -60 dBm.
	SignalStrength int16
}

// Scan schedules a network scan. It is asynchronous: the call returns once the
// scan is scheduled, and the station's Scanning property tracks progress (true
// while scanning, false when results are ready). iwd rejects a scan that is
// already in progress with a matchable Busy/InProgress error.
func (s *Station) Scan(ctx context.Context) error {
	if err := s.ensureInitialized(); err != nil {
		return WrapConnection("Station.ensureInitialized", err)
	}

	if _, err := s.call.Call(ctx, IwdStationIface, "Scan"); err != nil {
		return wrapIwdMethod(IwdStationIface, "Scan", err)
	}
	return nil
}

// GetOrderedNetworks returns the networks from the most recent scan, ordered by
// iwd (best signal first). No scan is required to read the last results.
func (s *Station) GetOrderedNetworks(ctx context.Context) ([]OrderedNetwork, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, WrapConnection("Station.ensureInitialized", err)
	}

	body, err := s.call.Call(ctx, IwdStationIface, "GetOrderedNetworks")
	if err != nil {
		return nil, wrapIwdMethod(IwdStationIface, "GetOrderedNetworks", err)
	}

	// The reply is a(on): an array of (network object path, int16 signal). godbus
	// maps the (on) struct to a Go struct by field order.
	var tuples []struct {
		Path   dbus.ObjectPath
		Signal int16
	}
	if err := dbus.Store(body, &tuples); err != nil {
		return nil, WrapVariant("GetOrderedNetworks", fmt.Errorf("unexpected reply shape: %w", err))
	}

	out := make([]OrderedNetwork, 0, len(tuples))
	for _, t := range tuples {
		out = append(out, OrderedNetwork{Network: t.Path, SignalStrength: t.Signal})
	}
	return out, nil
}

// SetAffinities sets the Affinities property, the BSS object paths the station
// should stay affine to. Affinities is an experimental read-write property and
// depends on driver support: hardware that cannot honor it makes iwd reject the
// write, which is surfaced as a matchable ErrNotSupported.
func (s *Station) SetAffinities(ctx context.Context, paths []string) error {
	if err := s.ensureInitialized(); err != nil {
		return WrapConnection("Station.ensureInitialized", err)
	}

	objPaths := make([]dbus.ObjectPath, 0, len(paths))
	for _, p := range paths {
		objPaths = append(objPaths, dbus.ObjectPath(p))
	}

	if err := s.call.SetProperty(ctx, IwdStationIface, "Affinities", objPaths); err != nil {
		return wrapIwdProperty(IwdStationIface, "Affinities", err)
	}
	return nil
}

// SubscribePropertiesChanged registers fn for raw station property-change signals.
func (s *Station) SubscribePropertiesChanged(ctx context.Context, fn func(StationPropertiesChanged)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePropertiesChanged: fn cannot be nil")
	}

	return s.signals.RegisterSignalHandlerWithUnsubscribe("org.freedesktop.DBus.Properties", "PropertiesChanged", func(sig *dbus.Signal) {
		if sig == nil || len(sig.Body) < 3 {
			return
		}

		iface, ok := sig.Body[0].(string)
		if !ok || iface != IwdStationIface {
			return
		}

		changed, ok := sig.Body[1].(map[string]dbus.Variant)
		if !ok {
			return
		}

		invalid, ok := sig.Body[2].([]string)
		if !ok {
			invalid = nil
		}

		fn(StationPropertiesChanged{
			Changed:     changed,
			Invalidated: invalid,
		})
	})
}

// SubscribeStateChanged registers fn for raw connection-state changes. An
// unparseable state is skipped rather than surfaced as StationStateUnknown.
func (s *Station) SubscribeStateChanged(ctx context.Context, fn func(StationState)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeStateChanged: fn cannot be nil")
	}

	return s.SubscribePropertiesChanged(ctx, func(ev StationPropertiesChanged) {
		variant, ok := ev.Changed["State"]
		if !ok {
			return
		}

		state, err := parseStationState(variant.Value())
		if err != nil {
			return
		}
		fn(state)
	})
}

// SubscribeScanningChanged registers fn for raw scanning-state changes.
func (s *Station) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeScanningChanged: fn cannot be nil")
	}

	return s.SubscribePropertiesChanged(ctx, func(ev StationPropertiesChanged) {
		variant, ok := ev.Changed["Scanning"]
		if !ok {
			return
		}

		b, ok := variant.Value().(bool)
		if ok {
			fn(b)
		}
	})
}

// Firehose emits high-frequency station signals for stress and integration tests.
func (s *Station) Firehose(ctx context.Context, fn func(FirehoseSignal)) error {
	if fn == nil {
		return fmt.Errorf("Firehose: fn cannot be nil")
	}

	// Wildcard interface ("*") + wildcard member ("*") gives all signals.
	return s.signals.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
		if sig == nil {
			return
		}

		iface, member := splitSignalName(sig.Name)
		fn(FirehoseSignal{
			ObjectPath: sig.Path,
			Interface:  iface,
			Member:     member,
			Body:       sig.Body,
			Raw:        sig,
		})
	})
}

// ensureInitialized verifies that s has been initialized by NewStation.
func (s *Station) ensureInitialized() error {
	if s.call == nil {
		return ErrStationUninitialized
	}
	return nil
}

// parseStationState normalizes the D-Bus State value into a StationState.
func parseStationState(v interface{}) (StationState, error) {
	str, ok := v.(string)
	if !ok {
		return StationStateUnknown, WrapVariant("State", fmt.Errorf("expected string, got %T", v))
	}
	state, ok := iwdvalue.ParseStationState(str)
	if !ok {
		return StationStateUnknown, WrapVariant("State", fmt.Errorf("invalid station state %q", str))
	}
	return state, nil
}

// parseStationObjectPath normalizes an optional station object-path property
// (ConnectedNetwork, ConnectedAccessPoint) into a *string. A nil value, empty
// path, or the root path "/" (iwd's "no object" sentinel) all yield nil. field
// names the property for error messages.
func parseStationObjectPath(field string, v interface{}) (*string, error) {
	var path string
	switch p := v.(type) {
	case nil:
		return nil, nil
	case dbus.ObjectPath:
		path = string(p)
	case string:
		path = p
	case dbus.Variant:
		return parseStationObjectPath(field, p.Value())
	default:
		return nil, WrapVariant(field, fmt.Errorf("expected object path, got %T", v))
	}

	if path == "" || path == "/" {
		return nil, nil
	}
	if !dbus.ObjectPath(path).IsValid() {
		return nil, WrapVariant(field, fmt.Errorf("invalid object path %q", path))
	}
	return &path, nil
}

// parseStationAffinities normalizes the optional Affinities array-of-object-path
// value into a []string, accepting both []dbus.ObjectPath and []interface{}
// forms. A nil value yields nil; a present-but-empty array yields an empty slice.
func parseStationAffinities(v interface{}) ([]string, error) {
	switch raw := v.(type) {
	case nil:
		return nil, nil
	case dbus.Variant:
		return parseStationAffinities(raw.Value())
	case []dbus.ObjectPath:
		out := make([]string, 0, len(raw))
		for _, p := range raw {
			if !p.IsValid() {
				return nil, WrapVariant("Affinities", fmt.Errorf("invalid object path %q", p))
			}
			out = append(out, string(p))
		}
		return out, nil
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, elem := range raw {
			s, ok := elem.(dbus.ObjectPath)
			if !ok {
				return nil, WrapVariant("Affinities", fmt.Errorf("expected object path, got %T", elem))
			}
			if !s.IsValid() {
				return nil, WrapVariant("Affinities", fmt.Errorf("invalid object path %q", s))
			}
			out = append(out, string(s))
		}
		return out, nil
	default:
		return nil, WrapVariant("Affinities", fmt.Errorf("expected object path array, got %T", v))
	}
}
