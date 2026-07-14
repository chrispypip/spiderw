package iwdbus

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"
)

// unknownPropertyRe matches the D-Bus error replies iwd returns when an
// optional Adapter property has no value. Real hardware reports "Getting
// property value failed" (the ELL property getter declined to provide a value);
// some paths report "GetProperty failed: unknown property". Only GetModel and
// GetVendor consult this, so the match is scoped to genuinely-optional
// properties where a getter failure means "absent".
// Matched case-insensitively since iwd/ELL casing of these messages is not
// guaranteed across versions.
var unknownPropertyRe = regexp.MustCompile(`(?i)GetProperty\sfailed:\sunknown property|Getting property value failed`)

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

// propertyCleared reports whether a PropertiesChanged event says the named
// property no longer has a value.
//
// iwd does not report "no longer connected" by sending the null path "/" in
// Changed — it lists the property in Invalidated and sends nothing else. This was
// confirmed on hardware: disconnecting a station produced no ConnectedNetwork or
// ConnectedAccessPoint value at all, and forgetting a network produced no
// KnownNetwork value. Any subscription over an optional property must therefore
// treat invalidation as the "gone" signal, or it silently never fires on the
// transition that matters most.
func propertyCleared(invalidated []string, name string) bool {
	return slices.Contains(invalidated, name)
}
