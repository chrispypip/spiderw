package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// adapterPath is the path of the primary mock adapter. It owns the device that
// the mock networks/BSSes/known networks hang under, and the firehose emitters
// target it.
const adapterPath = dbus.ObjectPath("/net/connman/iwd/phy0")

// adapter1Path is the path of a second mock adapter, exercising multi-adapter
// enumeration.
const adapter1Path = dbus.ObjectPath("/net/connman/iwd/phy1")

var exportedAdapters []*Adapter

// Adapter represents the mock iwd Adapter interface exported on D-Bus.
type Adapter struct {
	// Path is the D-Bus object path this adapter is exported at.
	Path dbus.ObjectPath

	// Powered is the mock Powered property.
	Powered bool

	// Name is the mock Name property.
	Name string

	// Model is the optional mock Model property.
	Model *string

	// Vendor is the optional mock Vendor property.
	Vendor *string

	// SupportedModes is the mock SupportedModes property.
	SupportedModes []string
}

// ExportAdapter exports the mock adapter objects on the D-Bus connection. Real
// systems can have several radios (built-in plus USB, multi-radio hardware), so
// the mock exports two adapters. model and vendor are the optional Model/Vendor
// of every exported adapter (nil under --omit-optionals).
func ExportAdapter(conn *dbus.Conn, model, vendor *string) error {
	adapters := []*Adapter{
		{
			Path:           adapterPath,
			Powered:        true,
			Name:           "phy0",
			Model:          model,
			Vendor:         vendor,
			SupportedModes: []string{"station", "ap"},
		},
		{
			Path:           adapter1Path,
			Powered:        true,
			Name:           "phy1",
			Model:          model,
			Vendor:         vendor,
			SupportedModes: []string{"station"},
		},
	}

	exportedAdapters = nil
	for _, a := range adapters {
		if err := conn.Export(a, a.Path, iwdbus.IwdAdapterIface); err != nil {
			return err
		}
		if err := conn.Export(a, a.Path, "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}
		if err := exportAdapterIntrospection(conn, a.Path); err != nil {
			return err
		}
		exportedAdapters = append(exportedAdapters, a)
	}

	return nil
}

func badModesPayload() interface{} {
	return []interface{}{true, 42, "not a mode"}
}

func (a *Adapter) buildPropertyMap() map[string]dbus.Variant {
	props := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(a.Powered),
		"Name":    dbus.MakeVariant(a.Name),
	}

	if *adapterBadModesFlag {
		props["SupportedModes"] = dbus.MakeVariant(badModesPayload())
	} else {
		props["SupportedModes"] = dbus.MakeVariant(a.SupportedModes)
	}

	if a.Model != nil {
		props["Model"] = dbus.MakeVariant(*a.Model)
	}
	if a.Vendor != nil {
		props["Vendor"] = dbus.MakeVariant(*a.Vendor)
	}
	return props
}

// GetAll returns all mock adapter properties for the requested interface.
func (a *Adapter) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdAdapterIface {
		return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}
	return a.buildPropertyMap(), nil
}

// Get returns a single mock adapter property for the requested interface.
func (a *Adapter) Get(iface, p string) (dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdAdapterIface {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}

	props := a.buildPropertyMap()
	v, ok := props[p]
	if !ok {
		// Reproduce real iwd: an absent *optional* (Model/Vendor) makes the
		// property getter fail with "Getting property value failed" rather than
		// being reported as an unknown property.
		if p == "Model" || p == "Vendor" {
			return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("getting property value failed"))
		}
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", p))
	}
	return v, nil
}

// Set stores supported mock adapter properties and emits matching change signals.
func (a *Adapter) Set(iface, p string, v dbus.Variant) *dbus.Error {
	if iface != iwdbus.IwdAdapterIface {
		return nil
	}

	switch p {
	case "Powered":
		b, ok := v.Value().(bool)
		if !ok {
			return dbus.MakeFailedError(fmt.Errorf("property Powered must be a bool, got %T", v))
		}
		a.Powered = b
		emitPropertiesChanged(a.Path, iwdbus.IwdAdapterIface, map[string]dbus.Variant{"Powered": dbus.MakeVariant(b)}, []string{})
		return nil
	default:
		return dbus.MakeFailedError(fmt.Errorf("cannot set property %q", p))
	}
}
