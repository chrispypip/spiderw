package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdAccessPointIface is the fully qualified D-Bus interface name for iwd access
// points. The AccessPoint interface is exported on a device object when the
// device is in AP mode, so an AccessPoint shares its object path with the Device
// (mutually exclusive with the Station interface).
const IwdAccessPointIface = IwdService + ".AccessPoint"

// AccessPointPropertiesChanged describes raw D-Bus access-point property-change
// data.
type AccessPointPropertiesChanged struct {
	// Changed contains raw D-Bus variants keyed by property name.
	Changed map[string]dbus.Variant

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// AccessPointProperties holds every access-point property read in a single
// Properties.GetAll call.
//
// Started is the only always-reported property. Everything else - including
// Scanning, which iwd's documentation calls mandatory - is optional: iwd omits it
// while the access point is not running, so a stopped AP reports Started and
// nothing else. Absent fields are left nil, or false for Scanning.
type AccessPointProperties struct {
	Started         bool
	Scanning        bool
	Name            *string
	Frequency       *uint32
	PairwiseCiphers []string
	GroupCipher     *string
}

// AccessPointOrderedNetwork is one entry of GetOrderedNetworks: a network the AP
// heard while scanning, with its signal strength and security type.
type AccessPointOrderedNetwork struct {
	// Name is the network SSID.
	Name string

	// SignalStrength is in units of 100 * dBm (iwd's native unit), e.g. -6000
	// means -60 dBm.
	SignalStrength int16

	// Type is the network security type (open, psk, 8021x).
	Type NetworkType
}

// AccessPoint wraps an iwd AccessPoint object using runtime introspection.
type AccessPoint struct {
	call    caller
	signals signalSource
}

// NewAccessPoint creates an AccessPoint for the given iwd object path (a device
// path, since the AccessPoint interface is exported on the device object).
func NewAccessPoint(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*AccessPoint, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdAccessPointIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdAccessPointIface)
	}
	return &AccessPoint{
		call:    caller(intro),
		signals: signalSource(intro),
	}, nil
}

// GetStarted reads the Started property.
func (a *AccessPoint) GetStarted(ctx context.Context) (bool, error) {
	if err := a.ensureInitialized(); err != nil {
		return false, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAccessPointIface, "Started")
	if err != nil {
		return false, WrapProperty(IwdAccessPointIface, "Started", err)
	}
	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Started", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// GetScanning reads the Scanning property. iwd documents it as always-present,
// but only exposes it (like Name/Frequency/ciphers) while the AP is started; a
// stopped AP omits it, so absence is reported as false (a stopped AP is not
// scanning) rather than an error.
func (a *AccessPoint) GetScanning(ctx context.Context) (bool, error) {
	if err := a.ensureInitialized(); err != nil {
		return false, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAccessPointIface, "Scanning")
	if err != nil {
		if isUnknownPropertyError(err) {
			return false, nil
		}
		return false, WrapProperty(IwdAccessPointIface, "Scanning", err)
	}
	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Scanning", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// GetName reads the optional Name property (the SSID while the AP is running). It
// returns a nil pointer (not an error) when iwd omits it.
func (a *AccessPoint) GetName(ctx context.Context) (*string, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAccessPointIface, "Name")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdAccessPointIface, "Name", err)
	}
	return parseOptionalAccessPointString("Name", value)
}

// GetFrequency reads the optional Frequency property (the operating frequency in
// MHz). It returns a nil pointer (not an error) when iwd omits it.
func (a *AccessPoint) GetFrequency(ctx context.Context) (*uint32, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAccessPointIface, "Frequency")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdAccessPointIface, "Frequency", err)
	}
	return parseAccessPointFrequency(value)
}

// GetPairwiseCiphers reads the optional PairwiseCiphers property. It returns nil
// (not an error) when iwd omits it.
func (a *AccessPoint) GetPairwiseCiphers(ctx context.Context) ([]string, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAccessPointIface, "PairwiseCiphers")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdAccessPointIface, "PairwiseCiphers", err)
	}
	return parseAccessPointCiphers(value)
}

// GetGroupCipher reads the optional GroupCipher property. It returns a nil
// pointer (not an error) when iwd omits it.
func (a *AccessPoint) GetGroupCipher(ctx context.Context) (*string, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAccessPointIface, "GroupCipher")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdAccessPointIface, "GroupCipher", err)
	}
	return parseOptionalAccessPointString("GroupCipher", value)
}

// GetProperties reads every access-point property in a single Properties.GetAll
// call. Only Started is always present. The remaining fields (including Scanning)
// are optional and left at their zero value (nil, or false for Scanning) when
// absent, which iwd does while the AP is not started.
//
// iwd's docs describe Scanning as always-present, but in practice it, like
// Name/Frequency/ciphers, only appears once the AP is started; a stopped AP
// reports only Started, so absent Scanning collapses to false.
func (a *AccessPoint) GetProperties(ctx context.Context) (*AccessPointProperties, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	raw, err := a.call.GetAll(ctx, IwdAccessPointIface)
	if err != nil {
		return nil, WrapProperty(IwdAccessPointIface, "GetAll", err)
	}

	props := &AccessPointProperties{}

	startedV, ok := raw["Started"]
	if !ok {
		return nil, WrapProperty(IwdAccessPointIface, "Started", fmt.Errorf("missing required property"))
	}
	started, ok := startedV.Value().(bool)
	if !ok {
		return nil, WrapVariant("Started", fmt.Errorf("expected bool, got %T", startedV.Value()))
	}
	props.Started = started

	if scanningV, ok := raw["Scanning"]; ok {
		scanning, ok := scanningV.Value().(bool)
		if !ok {
			return nil, WrapVariant("Scanning", fmt.Errorf("expected bool, got %T", scanningV.Value()))
		}
		props.Scanning = scanning
	}

	if v, ok := raw["Name"]; ok {
		name, err := parseOptionalAccessPointString("Name", v.Value())
		if err != nil {
			return nil, err
		}
		props.Name = name
	}
	if v, ok := raw["Frequency"]; ok {
		freq, err := parseAccessPointFrequency(v.Value())
		if err != nil {
			return nil, err
		}
		props.Frequency = freq
	}
	if v, ok := raw["PairwiseCiphers"]; ok {
		ciphers, err := parseAccessPointCiphers(v.Value())
		if err != nil {
			return nil, err
		}
		props.PairwiseCiphers = ciphers
	}
	if v, ok := raw["GroupCipher"]; ok {
		group, err := parseOptionalAccessPointString("GroupCipher", v.Value())
		if err != nil {
			return nil, err
		}
		props.GroupCipher = group
	}

	return props, nil
}

// Start starts a PSK-secured access point advertising ssid with passphrase psk.
// iwd returns matchable errors including AlreadyExists (an AP is already running),
// InProgress (busy), InvalidArguments, and Failed.
func (a *AccessPoint) Start(ctx context.Context, ssid, psk string) error {
	if err := a.ensureInitialized(); err != nil {
		return WrapConnection("AccessPoint.ensureInitialized", err)
	}

	if _, err := a.call.Call(ctx, IwdAccessPointIface, "Start", ssid, psk); err != nil {
		return wrapIwdMethod(IwdAccessPointIface, "Start", err)
	}
	return nil
}

// StartProfile starts an access point from the stored profile named ssid (a
// configuration file), which may configure security modes beyond PSK. iwd returns
// matchable errors including NotFound (no such profile), AlreadyExists,
// InProgress, InvalidArguments, and Failed.
func (a *AccessPoint) StartProfile(ctx context.Context, ssid string) error {
	if err := a.ensureInitialized(); err != nil {
		return WrapConnection("AccessPoint.ensureInitialized", err)
	}

	if _, err := a.call.Call(ctx, IwdAccessPointIface, "StartProfile", ssid); err != nil {
		return wrapIwdMethod(IwdAccessPointIface, "StartProfile", err)
	}
	return nil
}

// Stop stops the running access point.
func (a *AccessPoint) Stop(ctx context.Context) error {
	if err := a.ensureInitialized(); err != nil {
		return WrapConnection("AccessPoint.ensureInitialized", err)
	}

	if _, err := a.call.Call(ctx, IwdAccessPointIface, "Stop"); err != nil {
		return wrapIwdMethod(IwdAccessPointIface, "Stop", err)
	}
	return nil
}

// Scan schedules a scan from the access point (used to survey nearby networks).
// It is asynchronous: the Scanning property tracks progress. iwd returns matchable
// errors including NotSupported, NotAvailable, InProgress, and Failed.
func (a *AccessPoint) Scan(ctx context.Context) error {
	if err := a.ensureInitialized(); err != nil {
		return WrapConnection("AccessPoint.ensureInitialized", err)
	}

	if _, err := a.call.Call(ctx, IwdAccessPointIface, "Scan"); err != nil {
		return wrapIwdMethod(IwdAccessPointIface, "Scan", err)
	}
	return nil
}

// GetOrderedNetworks returns the networks from the most recent AP scan, ordered by
// iwd (best signal first). iwd returns NotAvailable when no scan data is present.
func (a *AccessPoint) GetOrderedNetworks(ctx context.Context) ([]AccessPointOrderedNetwork, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("AccessPoint.ensureInitialized", err)
	}

	body, err := a.call.Call(ctx, IwdAccessPointIface, "GetOrderedNetworks")
	if err != nil {
		return nil, wrapIwdMethod(IwdAccessPointIface, "GetOrderedNetworks", err)
	}

	// The reply is aa{sv}: an array of dicts, each with Name (s), SignalStrength
	// (n), and Type (s) - the security, e.g. "open"/"psk"/"8021x". (iwd's own docs
	// mislabel this key "Security"; the wire key is "Type", confirmed on hardware.)
	var entries []map[string]dbus.Variant
	if err := dbus.Store(body, &entries); err != nil {
		return nil, WrapVariant("GetOrderedNetworks", fmt.Errorf("unexpected reply shape: %w", err))
	}

	out := make([]AccessPointOrderedNetwork, 0, len(entries))
	for _, entry := range entries {
		net, err := parseAccessPointOrderedNetwork(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, net)
	}
	return out, nil
}

// SubscribePropertiesChanged registers fn for raw access-point property-change
// signals.
func (a *AccessPoint) SubscribePropertiesChanged(ctx context.Context, fn func(AccessPointPropertiesChanged)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePropertiesChanged: fn cannot be nil")
	}

	return a.signals.RegisterSignalHandlerWithUnsubscribe("org.freedesktop.DBus.Properties", "PropertiesChanged", func(sig *dbus.Signal) {
		if sig == nil || len(sig.Body) < 3 {
			return
		}

		iface, ok := sig.Body[0].(string)
		if !ok || iface != IwdAccessPointIface {
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

		fn(AccessPointPropertiesChanged{
			Changed:     changed,
			Invalidated: invalid,
		})
	})
}

// SubscribeStartedChanged registers fn for raw Started-state changes.
func (a *AccessPoint) SubscribeStartedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeStartedChanged: fn cannot be nil")
	}

	return a.SubscribePropertiesChanged(ctx, func(ev AccessPointPropertiesChanged) {
		variant, ok := ev.Changed["Started"]
		if !ok {
			return
		}
		if b, ok := variant.Value().(bool); ok {
			fn(b)
		}
	})
}

// SubscribeScanningChanged registers fn for raw scanning-state changes.
func (a *AccessPoint) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeScanningChanged: fn cannot be nil")
	}

	return a.SubscribePropertiesChanged(ctx, func(ev AccessPointPropertiesChanged) {
		variant, ok := ev.Changed["Scanning"]
		if !ok {
			return
		}
		if b, ok := variant.Value().(bool); ok {
			fn(b)
		}
	})
}

// Firehose emits high-frequency access-point signals for stress and integration
// tests.
func (a *AccessPoint) Firehose(ctx context.Context, fn func(FirehoseSignal)) error {
	if fn == nil {
		return fmt.Errorf("Firehose: fn cannot be nil")
	}

	return a.signals.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
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

// ensureInitialized verifies that a has been initialized by NewAccessPoint.
func (a *AccessPoint) ensureInitialized() error {
	if a.call == nil {
		return ErrAccessPointUninitialized
	}
	return nil
}

// parseOptionalAccessPointString normalizes an optional string property (Name,
// GroupCipher) into a *string. A nil or empty value yields nil.
func parseOptionalAccessPointString(field string, v interface{}) (*string, error) {
	switch s := v.(type) {
	case nil:
		return nil, nil
	case string:
		if s == "" {
			return nil, nil
		}
		return &s, nil
	case dbus.Variant:
		return parseOptionalAccessPointString(field, s.Value())
	default:
		return nil, WrapVariant(field, fmt.Errorf("expected string, got %T", v))
	}
}

// parseAccessPointFrequency normalizes the optional Frequency property into a
// *uint32. A nil value yields nil.
func parseAccessPointFrequency(v interface{}) (*uint32, error) {
	switch f := v.(type) {
	case nil:
		return nil, nil
	case uint32:
		return &f, nil
	case dbus.Variant:
		return parseAccessPointFrequency(f.Value())
	default:
		return nil, WrapVariant("Frequency", fmt.Errorf("expected uint32, got %T", v))
	}
}

// parseAccessPointCiphers normalizes the optional PairwiseCiphers array into a
// []string. A nil value yields nil.
func parseAccessPointCiphers(v interface{}) ([]string, error) {
	switch c := v.(type) {
	case nil:
		return nil, nil
	case []string:
		return c, nil
	case dbus.Variant:
		return parseAccessPointCiphers(c.Value())
	default:
		return nil, WrapVariant("PairwiseCiphers", fmt.Errorf("expected string array, got %T", v))
	}
}

// parseAccessPointOrderedNetwork normalizes one GetOrderedNetworks dict entry.
func parseAccessPointOrderedNetwork(m map[string]dbus.Variant) (AccessPointOrderedNetwork, error) {
	var net AccessPointOrderedNetwork

	if v, ok := m["Name"]; ok {
		s, ok := v.Value().(string)
		if !ok {
			return net, WrapVariant("Name", fmt.Errorf("expected string, got %T", v.Value()))
		}
		net.Name = s
	}
	if v, ok := m["SignalStrength"]; ok {
		sig, ok := v.Value().(int16)
		if !ok {
			return net, WrapVariant("SignalStrength", fmt.Errorf("expected int16, got %T", v.Value()))
		}
		net.SignalStrength = sig
	}
	if v, ok := m["Type"]; ok {
		s, ok := v.Value().(string)
		if !ok {
			return net, WrapVariant("Type", fmt.Errorf("expected string, got %T", v.Value()))
		}
		// A scanned neighbor can have a security iwd does not classify (an empty or
		// unrecognized Type), unlike a specific Network whose Type is strict. Leave
		// such an entry's Type as NetworkTypeUnknown rather than failing the whole
		// GetOrderedNetworks reply on one unclassifiable neighbor.
		if netType, err := parseNetworkType(s); err == nil {
			net.Type = netType
		}
	}

	return net, nil
}
