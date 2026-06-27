package mock

import (
	_ "embed"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

type introspectStub []byte

// Introspect returns static introspection XML for a mock D-Bus object.
func (i introspectStub) Introspect() (string, *dbus.Error) {
	return string(i), nil
}

// Introspection XML is embedded into the mock binary so it has no runtime
// dependency on the source tree layout or working directory.
var (
	//go:embed xml/daemon.xml
	daemonIntrospectionXML []byte

	//go:embed xml/adapter.xml
	adapterIntrospectionXML []byte

	//go:embed xml/device.xml
	deviceIntrospectionXML []byte

	//go:embed xml/bss.xml
	bssIntrospectionXML []byte

	//go:embed xml/network.xml
	networkIntrospectionXML []byte

	//go:embed xml/knownnetwork.xml
	knownNetworkIntrospectionXML []byte

	//go:embed xml/agentmanager.xml
	agentManagerIntrospectionXML []byte
)

func exportDaemonIntrospection(conn *dbus.Conn) error {
	xml := daemonIntrospectionXML
	if agentManagerExported() {
		// Splice the AgentManager interface block in before the closing </node>
		// so client introspection sees it alongside the Daemon interface.
		xml = []byte(strings.Replace(string(daemonIntrospectionXML), "</node>", string(agentManagerIntrospectionXML)+"</node>", 1))
	}
	return conn.Export(introspectStub(xml), iwdbus.IwdDaemonPath, iwdbus.DBusIntrospectableIface)
}

func exportAdapterIntrospection(conn *dbus.Conn, path dbus.ObjectPath) error {
	return conn.Export(introspectStub(adapterIntrospectionXML), path, iwdbus.DBusIntrospectableIface)
}

func exportDeviceIntrospection(conn *dbus.Conn, path dbus.ObjectPath) error {
	return conn.Export(introspectStub(deviceIntrospectionXML), path, iwdbus.DBusIntrospectableIface)
}

func exportBSSIntrospection(conn *dbus.Conn, path dbus.ObjectPath) error {
	return conn.Export(introspectStub(bssIntrospectionXML), path, iwdbus.DBusIntrospectableIface)
}

func exportNetworkIntrospection(conn *dbus.Conn, path dbus.ObjectPath) error {
	return conn.Export(introspectStub(networkIntrospectionXML), path, iwdbus.DBusIntrospectableIface)
}

func exportKnownNetworkIntrospection(conn *dbus.Conn, path dbus.ObjectPath) error {
	return conn.Export(introspectStub(knownNetworkIntrospectionXML), path, iwdbus.DBusIntrospectableIface)
}
