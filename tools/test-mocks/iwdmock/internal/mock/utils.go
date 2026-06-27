package mock

import (
	"flag"

	"github.com/godbus/dbus/v5"
)

var (
	omitDaemonFlag         = flag.Bool("omit-daemon", false, "Don't export daemon API")
	omitDaemonVersionFlag  = flag.Bool("omit-daemon-version", false, "Daemon.GetInfo omits the Version field")
	omitDaemonStateDirFlag = flag.Bool("omit-daemon-statedir", false, "Daemon.GetInfo omits the StateDirectory field")
	omitDaemonNetConfFlag  = flag.Bool("omit-daemon-netconf", false, "Daemon.GetInfo omits the NetworkConfigurationEnabled field")
	daemonBadVersionFlag   = flag.Bool("daemon-bad-version", false, "Daemon.GetInfo returns wrong type for Version")
	daemonBadStateDirFlag  = flag.Bool("daemon-bad-statedir", false, "Daemon.GetInfo returns wrong type for StateDirectory")
	daemonBadNetConfFlag   = flag.Bool("daemon-bad-netconf", false, "Daemon.GetInfo returns wrong type for NetworkConfigurationEnabled")
	daemonExtraFieldFlag   = flag.Bool("daemon-extra-field", false, "Daemon.GetInfo includes an extra unrecognized field")
	daemonBadPayloadFlag   = flag.Bool("daemon-bad-payload", false, "Daemon.GetInfo returns non-map (malformed DBus)")
	daemonFailCallsFlag    = flag.Bool("daemon-fail-calls", false, "Daemon.GetInfo always returns a DBus error")

	adapterBadModesFlag = flag.Bool("adapter-bad-modes", false, "Adapter.GetSupportedModes returns wrong type")

	omitDeviceFlag = flag.Bool("omit-device", false, "Don't export the device object")

	omitBSSFlag = flag.Bool("omit-bss", false, "Don't export the basic service set object")

	omitNetworkFlag = flag.Bool("omit-network", false, "Don't export the network objects")

	omitKnownNetworkFlag = flag.Bool("omit-knownnetwork", false, "Don't export the known-network objects")

	omitAgentFlag = flag.Bool("omit-agent", false, "Don't export the AgentManager interface (so agent registration is unavailable)")
)

func emitPropertiesChanged(path dbus.ObjectPath, iface string, changed map[string]dbus.Variant, invalid []string) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	_ = conn.Emit(path, "org.freedesktop.DBus.Properties.PropertiesChanged", iface, changed, invalid)
}
