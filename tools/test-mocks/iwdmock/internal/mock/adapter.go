package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

const adapterPath = dbus.ObjectPath("/net/connman/iwd/phy0")

var exportedAdapter *Adapter

// Adapter represents the mock iwd Adapter interface exported on D-Bus.
type Adapter struct {
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

// ExportAdapter exports the mock adapter object on the D-Bus connection.
func ExportAdapter(conn *dbus.Conn, model, vendor *string) error {
	a := &Adapter{
		Powered:        true,
		Name:           "phy0",
		Model:          model,
		Vendor:         vendor,
		SupportedModes: []string{"station", "ap"},
	}

	exportedAdapter = a

	// Export methods.
	if err := conn.Export(a, adapterPath, iwdbus.IwdAdapterIface); err != nil {
		return err
	}
	if err := conn.Export(a, adapterPath, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}
	return exportAdapterIntrospection(conn)
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
		emitPropertiesChanged(adapterPath, iwdbus.IwdAdapterIface, map[string]dbus.Variant{"Powered": dbus.MakeVariant(b)}, []string{})
		return nil
	default:
		return dbus.MakeFailedError(fmt.Errorf("cannot set property %q", p))
	}
}
