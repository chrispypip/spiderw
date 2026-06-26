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

// DevicePropertiesChanged describes device properties reported by a D-Bus
// PropertiesChanged signal. Changed contains the new values by property name;
// Invalidated contains property names whose values should be re-read if needed.
type DevicePropertiesChanged struct {
	// Changed contains new property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// DeviceProperties is a snapshot of all device properties read in a single
// D-Bus call. iwd reports all of these for every device.
type DeviceProperties struct {
	// Name is the device's human-friendly Name property.
	Name string

	// Address is the device's hardware (MAC) address.
	Address string

	// Powered reports whether the device is currently powered.
	Powered bool

	// Mode is the device's current operating mode.
	Mode Mode

	// Adapter is the object path of the adapter that owns this device. Resolve it
	// to a handle with Client.Adapter.
	Adapter string
}

// Device provides high-level operations for a specific iwd device object.
type Device struct {
	core core.DeviceIface
	path string
}

func newDevice(c core.DeviceIface, path string) *Device {
	if c == nil {
		return nil
	}
	return &Device{core: c, path: path}
}

// Path returns the D-Bus object path the device was constructed from.
//
// Path is static device identity, not an iwd property: it requires no D-Bus
// round-trip and never fails. Path returns "" for a nil receiver.
func (d *Device) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *Device) coreDevice(ctx context.Context, op string) (core.DeviceIface, error) {
	if d == nil || d.core == nil {
		logging.FromContext(ctx).Error(ctx, "device wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return d.core, nil
}

// Name returns the device name.
func (d *Device) Name(ctx context.Context) (string, error) {
	return delegate(ctx, "Device.Name", d.coreDevice, func(ctx context.Context, c core.DeviceIface) (string, error) {
		return c.Name(ctx)
	})
}

// Address returns the device's hardware (MAC) address.
func (d *Device) Address(ctx context.Context) (string, error) {
	return delegate(ctx, "Device.Address", d.coreDevice, func(ctx context.Context, c core.DeviceIface) (string, error) {
		return c.Address(ctx)
	})
}

// Powered reports whether the device is currently powered.
func (d *Device) Powered(ctx context.Context) (bool, error) {
	return delegate(ctx, "Device.Powered", d.coreDevice, func(ctx context.Context, c core.DeviceIface) (bool, error) {
		return c.Powered(ctx)
	})
}

// SetPowered changes whether the device is powered.
func (d *Device) SetPowered(ctx context.Context, powered bool) error {
	return do(ctx, "Device.SetPowered", d.coreDevice, func(ctx context.Context, c core.DeviceIface) error {
		return c.SetPowered(ctx, powered)
	})
}

// Mode returns the device's current operating mode.
func (d *Device) Mode(ctx context.Context) (Mode, error) {
	return delegate(ctx, "Device.Mode", d.coreDevice, func(ctx context.Context, c core.DeviceIface) (Mode, error) {
		cm, err := c.Mode(ctx)
		if err != nil {
			return ModeUnknown, err
		}
		return convertDeviceModeCoreToPublic(cm)
	})
}

// SetMode changes the device's operating mode. An unrecognized mode is rejected
// at the public boundary as an invalid argument.
func (d *Device) SetMode(ctx context.Context, mode Mode) error {
	return do(ctx, "Device.SetMode", d.coreDevice, func(ctx context.Context, c core.DeviceIface) error {
		cm, err := convertDeviceModePublicToCore(mode)
		if err != nil {
			return err
		}
		return c.SetMode(ctx, cm)
	})
}

// Adapter returns the object path of the adapter that owns this device.
//
// Resolve it to a handle with Client.Adapter.
func (d *Device) Adapter(ctx context.Context) (string, error) {
	return delegate(ctx, "Device.Adapter", d.coreDevice, func(ctx context.Context, c core.DeviceIface) (string, error) {
		return c.Adapter(ctx)
	})
}

// Properties reads every device property in a single D-Bus call
// (Properties.GetAll) instead of one call per property. Prefer it when you need
// several properties at once, such as building an overview of a device.
func (d *Device) Properties(ctx context.Context) (*DeviceProperties, error) {
	return delegate(ctx, "Device.Properties", d.coreDevice, func(ctx context.Context, c core.DeviceIface) (*DeviceProperties, error) {
		cp, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}

		mode, err := convertDeviceModeCoreToPublic(cp.Mode)
		if err != nil {
			return nil, err
		}

		return &DeviceProperties{
			Name:    cp.Name,
			Address: cp.Address,
			Powered: cp.Powered,
			Mode:    mode,
			Adapter: cp.Adapter,
		}, nil
	})
}

// SubscribePropertiesChanged registers fn for device property-change signals and
// returns a handle that unregisters the callback.
func (d *Device) SubscribePropertiesChanged(ctx context.Context, fn func(DevicePropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Device.SubscribePropertiesChanged"

	coreDevice, err := d.coreDevice(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceDevice, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreDevice.SubscribePropertiesChanged(ctx, func(core core.DevicePropertiesChanged) {
		changed := make(map[string]any, len(core.Changed))
		maps.Copy(changed, core.Changed)

		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if core.Invalidated != nil {
			invalidated = slices.Clone(core.Invalidated)
		}

		fn(DevicePropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribePoweredChanged registers fn for device powered-state changes and
// returns a handle that unregisters the callback.
func (d *Device) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Device.SubscribePoweredChanged"

	coreDevice, err := d.coreDevice(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceDevice, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreDevice.SubscribePoweredChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeModeChanged registers fn for device operating-mode changes and
// returns a handle that unregisters the callback.
func (d *Device) SubscribeModeChanged(ctx context.Context, fn func(Mode)) (UnsubscribeFunc, error) {
	const op = "Device.SubscribeModeChanged"

	coreDevice, err := d.coreDevice(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceDevice, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreDevice.SubscribeModeChanged(ctx, func(cm core.Mode) {
		// Lower layers only deliver recognized modes; drop anything else rather
		// than surfacing ModeUnknown to the caller.
		mode, err := convertDeviceModeCoreToPublic(cm)
		if err != nil {
			return
		}
		fn(mode)
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

func convertDeviceModeCoreToPublic(mode core.Mode) (Mode, error) {
	if !iwdvalue.ValidMode(mode) {
		details := fmt.Sprintf("invalid mode %q", mode)
		return ModeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceDevice, Op: "Device.convertMode", Details: details, Err: ErrInvalidArgument}
	}
	return Mode(mode), nil
}

func convertDeviceModePublicToCore(mode Mode) (core.Mode, error) {
	coreMode := core.Mode(mode)
	if !iwdvalue.ValidMode(coreMode) {
		details := fmt.Sprintf("invalid mode %q", mode)
		return core.ModeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceDevice, Op: "Device.convertMode", Details: details, Err: ErrInvalidArgument}
	}
	return coreMode, nil
}
