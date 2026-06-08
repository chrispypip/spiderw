package mock

import (
	_ "embed"

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
)

func exportDaemonIntrospection(conn *dbus.Conn) error {
	return conn.Export(introspectStub(daemonIntrospectionXML), iwdbus.IwdDaemonPath, iwdbus.DBusIntrospectableIface)
}

func exportAdapterIntrospection(conn *dbus.Conn) error {
	return conn.Export(introspectStub(adapterIntrospectionXML), adapterPath, iwdbus.DBusIntrospectableIface)
}
