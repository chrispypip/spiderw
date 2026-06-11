package iwdbus

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/godbus/dbus/v5"
)

// unknownPropertyRe matches the D-Bus error replies iwd returns when an
// optional Adapter property has no value. Real hardware reports "Getting
// property value failed" (the ELL property getter declined to provide a value);
// some paths report "GetProperty failed: unknown property". Only GetModel and
// GetVendor consult this, so the match is scoped to genuinely-optional
// properties where a getter failure means "absent".
var unknownPropertyRe = regexp.MustCompile(`GetProperty\sfailed:\sunknown property|Getting property value failed`)

// isUnknownPropertyError inspects a dbus error to decide whether the server is
// saying "that optional property has no value".
func isUnknownPropertyError(err error) bool {
	return unknownPropertyRe.MatchString(err.Error())
}

func parseOptionalString(v interface{}) (*string, error) {
	switch value := v.(type) {
	case nil:
		return nil, nil
	case string:
		return &value, nil
	case dbus.Variant:
		inner := value.Value()
		switch s := inner.(type) {
		case nil:
			return nil, nil
		case string:
			return &s, nil
		default:
			return nil, fmt.Errorf("expected string variant, got %T inside variant", inner)
		}
	default:
		return nil, fmt.Errorf("expected string or variant(string), got %T", v)
	}
}

func splitSignalName(name string) (string, string) {
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return name, ""
	}

	iface := strings.Join(parts[:len(parts)-1], ".")
	member := parts[len(parts)-1]
	return iface, member
}
