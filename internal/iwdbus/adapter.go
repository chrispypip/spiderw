package iwdbus

import (
	"context"
	"fmt"
	"slices"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// IwdAdapterIface is the fully qualified D-Bus interface name for iwd adapters.
const IwdAdapterIface = IwdService + ".Adapter"

// AdapterMode identifies a raw iwd adapter operating mode.
type AdapterMode = iwdvalue.AdapterMode

// Adapter mode constants identify raw iwd adapter modes.
// AdapterModeUnknown is reserved for invalid or unrecognized values.
const (
	// AdapterModeUnknown represents an invalid or unrecognized adapter mode.
	AdapterModeUnknown = iwdvalue.AdapterModeUnknown

	// AdapterModeStation is the iwd station adapter mode.
	AdapterModeStation = iwdvalue.AdapterModeStation

	// AdapterModeAP is the iwd access point adapter mode.
	AdapterModeAP = iwdvalue.AdapterModeAP

	// AdapterModeAdHoc is the iwd ad-hoc adapter mode.
	AdapterModeAdHoc = iwdvalue.AdapterModeAdHoc
)

// AdapterPropertiesChanged describes raw D-Bus adapter property-change data.
type AdapterPropertiesChanged struct {
	// Changed contains raw D-Bus variants keyed by property name.
	Changed map[string]dbus.Variant

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// Adapter wraps an iwd Adapter object using runtime introspection.
type Adapter struct {
	call    caller
	signals signalSource
}

// NewAdapter creates an Adapter for the given iwd object path.
func NewAdapter(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*Adapter, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdAdapterIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdAdapterIface)
	}
	return &Adapter{
		call:    caller(intro),
		signals: signalSource(intro),
	}, nil
}

// GetPowered reads the Powered property.
func (a *Adapter) GetPowered(ctx context.Context) (bool, error) {
	if err := a.ensureInitialized(); err != nil {
		return false, WrapConnection("Adapter.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAdapterIface, "Powered")
	if err != nil {
		return false, WrapProperty(IwdAdapterIface, "Powered", err)
	}

	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Powered", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// SetPowered sets the Powered property.
func (a *Adapter) SetPowered(ctx context.Context, val bool) error {
	if err := a.ensureInitialized(); err != nil {
		return WrapConnection("Adapter.ensureInitialized", err)
	}

	if err := a.call.SetProperty(ctx, IwdAdapterIface, "Powered", val); err != nil {
		return WrapProperty(IwdAdapterIface, "Powered", err)
	}
	return nil
}

// GetName reads the Name property.
func (a *Adapter) GetName(ctx context.Context) (string, error) {
	if err := a.ensureInitialized(); err != nil {
		return "", WrapConnection("Adapter.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAdapterIface, "Name")
	if err != nil {
		return "", WrapProperty(IwdAdapterIface, "Name", err)
	}

	s, ok := value.(string)
	if !ok {
		return "", WrapVariant("Name", fmt.Errorf("expected string, got %T", value))
	}
	// Empty/whitespace Name is a semantic concern owned by the core layer
	// (classified there as invalid state); the D-Bus layer returns the raw value.
	return s, nil
}

// GetModel reads the Model property.
func (a *Adapter) GetModel(ctx context.Context) (*string, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("Adapter.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAdapterIface, "Model")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdAdapterIface, "Model", err)
	}

	model, err := parseOptionalString(value)
	if err != nil {
		return nil, WrapVariant("Model", err)
	}
	return model, nil
}

// GetVendor reads the Vendor property.
func (a *Adapter) GetVendor(ctx context.Context) (*string, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("Adapter.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAdapterIface, "Vendor")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdAdapterIface, "Vendor", err)
	}

	vendor, err := parseOptionalString(value)
	if err != nil {
		return nil, WrapVariant("Vendor", err)
	}
	return vendor, nil
}

// GetSupportedModes reads and parses the SupportedModes property.
func (a *Adapter) GetSupportedModes(ctx context.Context) ([]AdapterMode, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("Adapter.ensureInitialized", err)
	}

	value, err := a.call.GetProperty(ctx, IwdAdapterIface, "SupportedModes")
	if err != nil {
		return nil, WrapProperty(IwdAdapterIface, "SupportedModes", err)
	}
	return parseSupportedModes(value)
}

// AdapterProperties holds every adapter property read in a single
// Properties.GetAll call. Model and Vendor are optional: a nil pointer means the
// adapter did not report a value (the property is simply absent from GetAll).
type AdapterProperties struct {
	Powered        bool
	Name           string
	Model          *string
	Vendor         *string
	SupportedModes []AdapterMode
}

// GetProperties reads every adapter property in a single Properties.GetAll call
// instead of one Get per property.
//
// Powered, Name, and SupportedModes are required; a missing one is an error.
// Model and Vendor are optional and left nil when absent from the reply, so the
// batched path needs no unknown-property handling.
func (a *Adapter) GetProperties(ctx context.Context) (*AdapterProperties, error) {
	if err := a.ensureInitialized(); err != nil {
		return nil, WrapConnection("Adapter.ensureInitialized", err)
	}

	raw, err := a.call.GetAll(ctx, IwdAdapterIface)
	if err != nil {
		return nil, WrapProperty(IwdAdapterIface, "GetAll", err)
	}

	props := &AdapterProperties{}

	poweredV, ok := raw["Powered"]
	if !ok {
		return nil, WrapProperty(IwdAdapterIface, "Powered", fmt.Errorf("missing required property"))
	}
	powered, ok := poweredV.Value().(bool)
	if !ok {
		return nil, WrapVariant("Powered", fmt.Errorf("expected bool, got %T", poweredV.Value()))
	}
	props.Powered = powered

	nameV, ok := raw["Name"]
	if !ok {
		return nil, WrapProperty(IwdAdapterIface, "Name", fmt.Errorf("missing required property"))
	}
	// Empty/whitespace Name is a semantic concern owned by the core layer; the
	// D-Bus layer returns the raw value (matching GetName).
	name, ok := nameV.Value().(string)
	if !ok {
		return nil, WrapVariant("Name", fmt.Errorf("expected string, got %T", nameV.Value()))
	}
	props.Name = name

	modesV, ok := raw["SupportedModes"]
	if !ok {
		return nil, WrapProperty(IwdAdapterIface, "SupportedModes", fmt.Errorf("missing required property"))
	}
	modes, err := parseSupportedModes(modesV.Value())
	if err != nil {
		return nil, err
	}
	props.SupportedModes = modes

	if modelV, ok := raw["Model"]; ok {
		model, err := parseOptionalString(modelV.Value())
		if err != nil {
			return nil, WrapVariant("Model", err)
		}
		props.Model = model
	}

	if vendorV, ok := raw["Vendor"]; ok {
		vendor, err := parseOptionalString(vendorV.Value())
		if err != nil {
			return nil, WrapVariant("Vendor", err)
		}
		props.Vendor = vendor
	}

	return props, nil
}

// SupportsMode reports whether the adapter declares support for mode.
func (a *Adapter) SupportsMode(ctx context.Context, mode AdapterMode) (bool, error) {
	if err := a.ensureInitialized(); err != nil {
		return false, WrapConnection("Adapter.ensureInitialized", err)
	}

	if mode == AdapterModeUnknown {
		return false, fmt.Errorf("invalid mode: %s", mode.String())
	}
	modes, err := a.GetSupportedModes(ctx)
	if err != nil {
		return false, err
	}
	return isModeSupported(modes, mode)
}

// SupportsStation is a convenience wrapper for SupportsMode(AdapterModeStation).
func (a *Adapter) SupportsStation(ctx context.Context) (bool, error) {
	return a.SupportsMode(ctx, AdapterModeStation)
}

// SupportsAP is a convenience wrapper for SupportsMode(AdapterModeAP).
func (a *Adapter) SupportsAP(ctx context.Context) (bool, error) {
	return a.SupportsMode(ctx, AdapterModeAP)
}

// SupportsAdHoc is a convenience wrapper for SupportsMode(AdapterModeAdHoc).
func (a *Adapter) SupportsAdHoc(ctx context.Context) (bool, error) {
	return a.SupportsMode(ctx, AdapterModeAdHoc)
}

// SubscribePropertiesChanged registers fn for raw adapter property-change signals.
func (a *Adapter) SubscribePropertiesChanged(ctx context.Context, fn func(AdapterPropertiesChanged)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePropertiesChanged: fn cannot be nil")
	}

	return a.signals.RegisterSignalHandlerWithUnsubscribe("org.freedesktop.DBus.Properties", "PropertiesChanged", func(sig *dbus.Signal) {
		if sig == nil || len(sig.Body) < 3 {
			return
		}

		iface, ok := sig.Body[0].(string)
		if !ok || iface != IwdAdapterIface {
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

		fn(AdapterPropertiesChanged{
			Changed:     changed,
			Invalidated: invalid,
		})
	})
}

// SubscribePoweredChanged registers fn for raw powered-state changes.
func (a *Adapter) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePoweredChanged: fn cannot be nil")
	}

	return a.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
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

// Firehose emits high-frequency adapter signals for stress and integration tests.
func (a *Adapter) Firehose(ctx context.Context, fn func(FirehoseSignal)) error {
	if fn == nil {
		return fmt.Errorf("Firehose: fn cannot be nil")
	}

	// Wildcard interface ("*") + wildcard member ("*") gives all signals.
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

// ParseAdapterMode converts a canonical iwd mode string to an AdapterMode.
func ParseAdapterMode(s string) (AdapterMode, error) {
	mode, ok := iwdvalue.ParseAdapterMode(s)
	if !ok {
		return AdapterModeUnknown, fmt.Errorf("invalid adapter mode %q", s)
	}
	return mode, nil
}

// ensureInitialized verifies that a has been initialized by NewAdapter.
func (a *Adapter) ensureInitialized() error {
	if a.call == nil {
		return ErrAdapterUninitialized
	}
	return nil
}

func parseSupportedModes(v interface{}) ([]AdapterMode, error) {
	switch raw := v.(type) {
	case []string:
		out := make([]AdapterMode, 0, len(raw))
		for _, s := range raw {
			mode, ok := iwdvalue.ParseAdapterMode(s)
			if !ok {
				return nil, WrapVariant("SupportedModes", fmt.Errorf("invalid mode %q", s))
			}
			out = append(out, mode)
		}
		return out, nil
	case []interface{}:
		out := make([]AdapterMode, 0, len(raw))
		for _, elem := range raw {
			str, ok := elem.(string)
			if !ok {
				return nil, WrapVariant("SupportedModes", fmt.Errorf("expected string, got %T", elem))
			}
			mode, ok := iwdvalue.ParseAdapterMode(str)
			if !ok {
				return nil, WrapVariant("SupportedModes", fmt.Errorf("invalid mode %q", str))
			}
			out = append(out, mode)
		}
		return out, nil
	default:
		return nil, WrapVariant("SupportedModes", fmt.Errorf("unexpected type %T", v))
	}
}

func isModeSupported(modes []AdapterMode, mode AdapterMode) (bool, error) {
	if !iwdvalue.ValidAdapterMode(mode) {
		return false, fmt.Errorf("invalid mode: %s", mode.String())
	}
	return slices.Contains(modes, mode), nil
}
