//go:build unit || stress || fuzz || race

package iwdbus

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

type introspectionNode struct {
	XMLName xml.Name            `xml:"node"`
	Nodes   []introspectionNode `xml:"node"`
	Name    string              `xml:"name,attr"`
}

func introspectChildNames(ctx context.Context, conn *dbus.Conn, service string, path dbus.ObjectPath) ([]string, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil dbus conn")
	}

	obj := conn.Object(service, path)
	call := obj.CallWithContext(ctx, DBusIntrospectFn, 0)
	if call.Err != nil {
		return nil, call.Err
	}
	if len(call.Body) != 1 {
		return nil, fmt.Errorf("unexpected introspection body length: %d", len(call.Body))
	}
	s, ok := call.Body[0].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected introspection body type %T", call.Body[0])
	}

	return parseIntrospectionChildNames(s)
}

func parseIntrospectionChildNames(xmlStr string) ([]string, error) {
	var root introspectionNode
	if err := xml.Unmarshal([]byte(xmlStr), &root); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(root.Nodes))
	for _, n := range root.Nodes {
		name := strings.TrimSpace(n.Name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}
