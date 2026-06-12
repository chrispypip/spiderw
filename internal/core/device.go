package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// DevicePropertiesChanged describes normalized device property-change data.
type DevicePropertiesChanged struct {
	// Changed contains normalized property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

type deviceRaw interface {
	GetName(ctx context.Context) (string, error)
	GetAddress(ctx context.Context) (string, error)
	GetPowered(ctx context.Context) (bool, error)
	SetPowered(ctx context.Context, powered bool) error
	GetMode(ctx context.Context) (iwdbus.Mode, error)
	SetMode(ctx context.Context, mode iwdbus.Mode) error
	GetAdapter(ctx context.Context) (dbus.ObjectPath, error)
	GetProperties(ctx context.Context) (*iwdbus.DeviceProperties, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.DevicePropertiesChanged)) (iwdbus.UnsubscribeFunc, error)
	SubscribePoweredChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
	SubscribeModeChanged(ctx context.Context, fn func(iwdbus.Mode)) (iwdbus.UnsubscribeFunc, error)
}

// DeviceIface defines the core device operations used by the public layer.
type DeviceIface interface {
	Name(ctx context.Context) (string, error)
	Address(ctx context.Context) (string, error)
	Powered(ctx context.Context) (bool, error)
	SetPowered(ctx context.Context, powered bool) error
	Mode(ctx context.Context) (Mode, error)
	SetMode(ctx context.Context, mode Mode) error
	Adapter(ctx context.Context) (string, error)
	Properties(ctx context.Context) (*DeviceProperties, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(DevicePropertiesChanged)) (UnsubscribeFunc, error)
	SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
	SubscribeModeChanged(ctx context.Context, fn func(Mode)) (UnsubscribeFunc, error)
}

// DeviceProperties holds normalized device properties read in a single backend
// call. iwd reports all of these for every device, so none are optional.
type DeviceProperties struct {
	Name    string
	Address string
	Powered bool
	Mode    Mode
	Adapter string
}

// Device is the core-layer facade over a raw iwd device backend.
type Device struct {
	raw deviceRaw
}

// NewDevice wraps a raw device backend in a core-layer Device.
func NewDevice(raw deviceRaw) *Device {
	if raw == nil {
		return nil
	}
	return &Device{raw: raw}
}

func (d *Device) rawDevice(op string) (deviceRaw, error) {
	if d == nil || d.raw == nil {
		return nil, WrapInvalidState(ResourceDevice, op, "device wrapper was nil", ErrDeviceNotInitialized)
	}
	return d.raw, nil
}

// Name returns the normalized device name.
func (d *Device) Name(ctx context.Context) (string, error) {
	const op = "Device.Name"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return "", err
	}

	raw, err := rawDevice.GetName(ctx)
	if err != nil {
		return "", WrapDeviceUnavailable(op, "failed querying iwd Device name", err)
	}

	n := strings.TrimSpace(raw)
	if n == "" {
		return "", WrapInvalidState(ResourceDevice, op, "device returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	return n, nil
}

// Address returns the normalized device hardware (MAC) address.
func (d *Device) Address(ctx context.Context) (string, error) {
	const op = "Device.Address"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return "", err
	}

	raw, err := rawDevice.GetAddress(ctx)
	if err != nil {
		return "", WrapDeviceUnavailable(op, "failed querying iwd Device address", err)
	}

	a := strings.TrimSpace(raw)
	if a == "" {
		return "", WrapInvalidState(ResourceDevice, op, "device returned empty Address", fmt.Errorf("missing or invalid Address field"))
	}

	return a, nil
}

// Powered returns the normalized device powered state.
func (d *Device) Powered(ctx context.Context) (bool, error) {
	const op = "Device.Powered"

	raw, err := d.rawDevice(op)
	if err != nil {
		return false, err
	}

	value, err := raw.GetPowered(ctx)
	if err != nil {
		return false, WrapDeviceUnavailable(op, "failed querying iwd Device powered", err)
	}

	return value, nil
}

// SetPowered sets the device powered state through the raw backend.
func (d *Device) SetPowered(ctx context.Context, powered bool) error {
	const op = "Device.SetPowered"

	raw, err := d.rawDevice(op)
	if err != nil {
		return err
	}

	if err := raw.SetPowered(ctx, powered); err != nil {
		return WrapDeviceUnavailable(op, "failed setting iwd Device powered", err)
	}

	return nil
}

// Mode returns the normalized device operating mode.
func (d *Device) Mode(ctx context.Context) (Mode, error) {
	const op = "Device.Mode"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return ModeUnknown, err
	}

	raw, err := rawDevice.GetMode(ctx)
	if err != nil {
		return ModeUnknown, WrapDeviceUnavailable(op, "failed querying iwd Device mode", err)
	}

	return validateMode(op, raw)
}

// SetMode sets the device operating mode through the raw backend.
func (d *Device) SetMode(ctx context.Context, mode Mode) error {
	const op = "Device.SetMode"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return err
	}

	if !iwdvalue.ValidMode(mode) {
		err := fmt.Errorf("invalid mode %q", mode)
		return WrapInvalidArgument(ResourceDevice, op, "unknown mode", err)
	}

	if err := rawDevice.SetMode(ctx, mode); err != nil {
		return WrapDeviceUnavailable(op, "failed setting iwd Device mode", err)
	}

	return nil
}

// Adapter returns the object path of the adapter that owns this device.
func (d *Device) Adapter(ctx context.Context) (string, error) {
	const op = "Device.Adapter"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return "", err
	}

	raw, err := rawDevice.GetAdapter(ctx)
	if err != nil {
		return "", WrapDeviceUnavailable(op, "failed querying iwd Device adapter", err)
	}

	path := strings.TrimSpace(string(raw))
	if path == "" {
		return "", WrapInvalidState(ResourceDevice, op, "device returned empty Adapter", fmt.Errorf("missing or invalid Adapter field"))
	}

	return path, nil
}

// Properties returns all normalized device properties read in a single backend
// call (Properties.GetAll), applying the same normalization as the per-property
// getters: Name/Address/Adapter are trimmed and required, and Mode is validated.
func (d *Device) Properties(ctx context.Context) (*DeviceProperties, error) {
	const op = "Device.Properties"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawDevice.GetProperties(ctx)
	if err != nil {
		return nil, WrapDeviceUnavailable(op, "failed querying iwd Device properties", err)
	}

	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, WrapInvalidState(ResourceDevice, op, "device returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	address := strings.TrimSpace(raw.Address)
	if address == "" {
		return nil, WrapInvalidState(ResourceDevice, op, "device returned empty Address", fmt.Errorf("missing or invalid Address field"))
	}

	adapter := strings.TrimSpace(string(raw.Adapter))
	if adapter == "" {
		return nil, WrapInvalidState(ResourceDevice, op, "device returned empty Adapter", fmt.Errorf("missing or invalid Adapter field"))
	}

	mode, err := validateMode(op, raw.Mode)
	if err != nil {
		return nil, err
	}

	return &DeviceProperties{
		Name:    name,
		Address: address,
		Powered: raw.Powered,
		Mode:    mode,
		Adapter: adapter,
	}, nil
}

// SubscribePropertiesChanged registers fn for normalized property-change events.
func (d *Device) SubscribePropertiesChanged(ctx context.Context, fn func(DevicePropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Device.SubscribePropertiesChanged"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceDevice, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawDevice.SubscribePropertiesChanged(ctx, func(raw iwdbus.DevicePropertiesChanged) {
		changed := make(map[string]any, len(raw.Changed))
		for k, v := range raw.Changed {
			changed[k] = v.Value()
		}
		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if raw.Invalidated != nil {
			invalidated = slices.Clone(raw.Invalidated)
		}

		fn(DevicePropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, WrapDeviceUnavailable(op, "failed to call iwd Device subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribePoweredChanged registers fn for normalized powered-state events.
func (d *Device) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Device.SubscribePoweredChanged"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceDevice, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawDevice.SubscribePoweredChanged(ctx, fn)
	if err != nil {
		return nil, WrapDeviceUnavailable(op, "failed to call iwd Device subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeModeChanged registers fn for normalized operating-mode events.
func (d *Device) SubscribeModeChanged(ctx context.Context, fn func(Mode)) (UnsubscribeFunc, error) {
	const op = "Device.SubscribeModeChanged"

	rawDevice, err := d.rawDevice(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceDevice, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawDevice.SubscribeModeChanged(ctx, fn)
	if err != nil {
		return nil, WrapDeviceUnavailable(op, "failed to call iwd Device subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// validateMode ensures the backend reported a recognized iwd mode, treating an
// unknown value as invalid state rather than silently propagating it.
func validateMode(op string, mode iwdbus.Mode) (Mode, error) {
	if !iwdvalue.ValidMode(mode) {
		details := fmt.Sprintf("device reported unknown mode %q", mode)
		return ModeUnknown, WrapInvalidState(ResourceDevice, op, details, fmt.Errorf("missing or invalid Mode field"))
	}
	return mode, nil
}
