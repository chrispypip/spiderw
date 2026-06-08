package iwdbus

import "github.com/godbus/dbus/v5"

// FirehoseSignal describes a synthetic signal emitted by a firehose test source.
type FirehoseSignal struct {
	// ObjectPath is the D-Bus object path that emitted the signal.
	ObjectPath dbus.ObjectPath

	// Interface is the signal interface name.
	Interface string

	// Member is the signal member name.
	Member string

	// Body is the raw signal body.
	Body []interface{}

	// Raw is the original D-Bus signal.
	Raw *dbus.Signal
}
