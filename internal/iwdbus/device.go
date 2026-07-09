package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdDeviceIface is the fully qualified D-Bus interface name for iwd devices.
const IwdDeviceIface = IwdService + ".Device"

// DevicePropertiesChanged describes raw D-Bus device property-change data.
type DevicePropertiesChanged struct {
	// Changed contains raw D-Bus variants keyed by property name.
	Changed map[string]dbus.Variant

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// Device wraps an iwd Device object using runtime introspection.
type Device struct {
	call    caller
	signals signalSource
}

// NewDevice creates a Device for the given iwd object path.
func NewDevice(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*Device, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdDeviceIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdDeviceIface)
	}
	return &Device{
		call:    caller(intro),
		signals: signalSource(intro),
	}, nil
}

// GetName reads the Name property.
func (d *Device) GetName(ctx context.Context) (string, error) {
	if err := d.ensureInitialized(); err != nil {
		return "", WrapConnection("Device.ensureInitialized", err)
	}

	value, err := d.call.GetProperty(ctx, IwdDeviceIface, "Name")
	if err != nil {
		return "", WrapProperty(IwdDeviceIface, "Name", err)
	}

	s, ok := value.(string)
	if !ok {
		return "", WrapVariant("Name", fmt.Errorf("expected string, got %T", value))
	}
	// Empty/whitespace Name is a semantic concern owned by the core layer; the
	// D-Bus layer returns the raw value.
	return s, nil
}

// GetAddress reads the Address (hardware MAC) property.
func (d *Device) GetAddress(ctx context.Context) (string, error) {
	if err := d.ensureInitialized(); err != nil {
		return "", WrapConnection("Device.ensureInitialized", err)
	}

	value, err := d.call.GetProperty(ctx, IwdDeviceIface, "Address")
	if err != nil {
		return "", WrapProperty(IwdDeviceIface, "Address", err)
	}

	s, ok := value.(string)
	if !ok {
		return "", WrapVariant("Address", fmt.Errorf("expected string, got %T", value))
	}
	return s, nil
}

// GetPowered reads the Powered property.
func (d *Device) GetPowered(ctx context.Context) (bool, error) {
	if err := d.ensureInitialized(); err != nil {
		return false, WrapConnection("Device.ensureInitialized", err)
	}

	value, err := d.call.GetProperty(ctx, IwdDeviceIface, "Powered")
	if err != nil {
		return false, WrapProperty(IwdDeviceIface, "Powered", err)
	}

	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Powered", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// SetPowered sets the Powered property.
func (d *Device) SetPowered(ctx context.Context, val bool) error {
	if err := d.ensureInitialized(); err != nil {
		return WrapConnection("Device.ensureInitialized", err)
	}

	if err := d.call.SetProperty(ctx, IwdDeviceIface, "Powered", val); err != nil {
		return wrapIwdProperty(IwdDeviceIface, "Powered", err)
	}
	return nil
}

// GetMode reads and parses the Mode property.
func (d *Device) GetMode(ctx context.Context) (Mode, error) {
	if err := d.ensureInitialized(); err != nil {
		return ModeUnknown, WrapConnection("Device.ensureInitialized", err)
	}

	value, err := d.call.GetProperty(ctx, IwdDeviceIface, "Mode")
	if err != nil {
		return ModeUnknown, WrapProperty(IwdDeviceIface, "Mode", err)
	}
	return parseDeviceMode(value)
}

// SetMode sets the Mode property to the given canonical iwd mode.
func (d *Device) SetMode(ctx context.Context, mode Mode) error {
	if err := d.ensureInitialized(); err != nil {
		return WrapConnection("Device.ensureInitialized", err)
	}

	if mode == ModeUnknown {
		return WrapProperty(IwdDeviceIface, "Mode", fmt.Errorf("invalid mode: %s", mode.String()))
	}

	if err := d.call.SetProperty(ctx, IwdDeviceIface, "Mode", mode.String()); err != nil {
		return wrapIwdProperty(IwdDeviceIface, "Mode", err)
	}
	return nil
}

// GetAdapter reads the Adapter property, the object path of the adapter that
// owns this device.
func (d *Device) GetAdapter(ctx context.Context) (dbus.ObjectPath, error) {
	if err := d.ensureInitialized(); err != nil {
		return "", WrapConnection("Device.ensureInitialized", err)
	}

	value, err := d.call.GetProperty(ctx, IwdDeviceIface, "Adapter")
	if err != nil {
		return "", WrapProperty(IwdDeviceIface, "Adapter", err)
	}
	return parseObjectPath(value)
}

// DeviceProperties holds every device property read in a single
// Properties.GetAll call. iwd reports all of these for every device, so each is
// required.
type DeviceProperties struct {
	Name    string
	Address string
	Powered bool
	Mode    Mode
	Adapter dbus.ObjectPath
}

// GetProperties reads every device property in a single Properties.GetAll call
// instead of one Get per property. All properties are required; a missing one is
// an error.
func (d *Device) GetProperties(ctx context.Context) (*DeviceProperties, error) {
	if err := d.ensureInitialized(); err != nil {
		return nil, WrapConnection("Device.ensureInitialized", err)
	}

	raw, err := d.call.GetAll(ctx, IwdDeviceIface)
	if err != nil {
		return nil, WrapProperty(IwdDeviceIface, "GetAll", err)
	}

	props := &DeviceProperties{}

	nameV, ok := raw["Name"]
	if !ok {
		return nil, WrapProperty(IwdDeviceIface, "Name", fmt.Errorf("missing required property"))
	}
	// Empty/whitespace Name is a semantic concern owned by the core layer; the
	// D-Bus layer returns the raw value (matching GetName).
	name, ok := nameV.Value().(string)
	if !ok {
		return nil, WrapVariant("Name", fmt.Errorf("expected string, got %T", nameV.Value()))
	}
	props.Name = name

	addressV, ok := raw["Address"]
	if !ok {
		return nil, WrapProperty(IwdDeviceIface, "Address", fmt.Errorf("missing required property"))
	}
	address, ok := addressV.Value().(string)
	if !ok {
		return nil, WrapVariant("Address", fmt.Errorf("expected string, got %T", addressV.Value()))
	}
	props.Address = address

	poweredV, ok := raw["Powered"]
	if !ok {
		return nil, WrapProperty(IwdDeviceIface, "Powered", fmt.Errorf("missing required property"))
	}
	powered, ok := poweredV.Value().(bool)
	if !ok {
		return nil, WrapVariant("Powered", fmt.Errorf("expected bool, got %T", poweredV.Value()))
	}
	props.Powered = powered

	modeV, ok := raw["Mode"]
	if !ok {
		return nil, WrapProperty(IwdDeviceIface, "Mode", fmt.Errorf("missing required property"))
	}
	mode, err := parseDeviceMode(modeV.Value())
	if err != nil {
		return nil, err
	}
	props.Mode = mode

	adapterV, ok := raw["Adapter"]
	if !ok {
		return nil, WrapProperty(IwdDeviceIface, "Adapter", fmt.Errorf("missing required property"))
	}
	adapter, err := parseObjectPath(adapterV.Value())
	if err != nil {
		return nil, err
	}
	props.Adapter = adapter

	return props, nil
}

// SubscribePropertiesChanged registers fn for raw device property-change signals.
func (d *Device) SubscribePropertiesChanged(ctx context.Context, fn func(DevicePropertiesChanged)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePropertiesChanged: fn cannot be nil")
	}

	return d.signals.RegisterSignalHandlerWithUnsubscribe("org.freedesktop.DBus.Properties", "PropertiesChanged", func(sig *dbus.Signal) {
		if sig == nil || len(sig.Body) < 3 {
			return
		}

		iface, ok := sig.Body[0].(string)
		if !ok || iface != IwdDeviceIface {
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

		fn(DevicePropertiesChanged{
			Changed:     changed,
			Invalidated: invalid,
		})
	})
}

// SubscribePoweredChanged registers fn for raw powered-state changes.
func (d *Device) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePoweredChanged: fn cannot be nil")
	}

	return d.SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
		variant, ok := ev.Changed["Powered"]
		if !ok {
			return
		}

		b, ok := variant.Value().(bool)
		if ok {
			fn(b)
		}
	})
}

// SubscribeModeChanged registers fn for raw operating-mode changes. Mode is
// read-write, so iwd emits a PropertiesChanged whenever a device switches mode.
// An unparsable mode is skipped rather than surfaced as ModeUnknown.
func (d *Device) SubscribeModeChanged(ctx context.Context, fn func(Mode)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeModeChanged: fn cannot be nil")
	}

	return d.SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
		variant, ok := ev.Changed["Mode"]
		if !ok {
			return
		}

		mode, err := parseDeviceMode(variant.Value())
		if err != nil {
			return
		}
		fn(mode)
	})
}

// Firehose emits high-frequency device signals for stress and integration tests.
func (d *Device) Firehose(ctx context.Context, fn func(FirehoseSignal)) error {
	if fn == nil {
		return fmt.Errorf("Firehose: fn cannot be nil")
	}

	// Wildcard interface ("*") + wildcard member ("*") gives all signals.
	return d.signals.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
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

// ensureInitialized verifies that d has been initialized by NewDevice.
func (d *Device) ensureInitialized() error {
	if d.call == nil {
		return ErrDeviceUninitialized
	}
	return nil
}

// parseDeviceMode normalizes the D-Bus Mode value into an Mode. The iwd
// Device.Mode property uses the same canonical mode strings as the adapter.
func parseDeviceMode(v interface{}) (Mode, error) {
	s, ok := v.(string)
	if !ok {
		return ModeUnknown, WrapVariant("Mode", fmt.Errorf("expected string, got %T", v))
	}
	mode, err := ParseMode(s)
	if err != nil {
		return ModeUnknown, WrapVariant("Mode", err)
	}
	return mode, nil
}

// parseObjectPath normalizes a D-Bus object-path value, accepting both the typed
// dbus.ObjectPath and a plain string form.
func parseObjectPath(v interface{}) (dbus.ObjectPath, error) {
	switch p := v.(type) {
	case dbus.ObjectPath:
		if !p.IsValid() {
			return "", WrapVariant("Adapter", fmt.Errorf("invalid object path %q", p))
		}
		return p, nil
	case string:
		path := dbus.ObjectPath(p)
		if !path.IsValid() {
			return "", WrapVariant("Adapter", fmt.Errorf("invalid object path %q", p))
		}
		return path, nil
	default:
		return "", WrapVariant("Adapter", fmt.Errorf("expected object path, got %T", v))
	}
}
