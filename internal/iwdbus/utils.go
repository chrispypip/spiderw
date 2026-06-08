package iwdbus

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/godbus/dbus/v5"
)

var re = regexp.MustCompile(`GetProperty\sfailed:\sunknown property`)

// isUnknownPropertyError inspects a wrapped dbus.Error to decide whether the
// server is saying "that property does not exist" for an optional property.
func isUnknownPropertyError(err error) bool {
	return re.MatchString(err.Error())
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
