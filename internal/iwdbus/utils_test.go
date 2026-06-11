//go:build unit

package iwdbus

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsUnknownPropertyError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		msg  string
		want bool
	}{
		{"real iwd getter failure", "Getting property value failed", true},
		{"lowercase getter failure", "getting property value failed", true},
		{"wrapped real iwd getter failure", `dbus property error: iface=net.connman.iwd.Adapter, property=Model: Getting property value failed`, true},
		{"unknown property", `GetProperty failed: unknown property "Model"`, true},
		{"unrelated dbus failure", "connection reset by peer", false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isUnknownPropertyError(errors.New(tc.msg)))
		})
	}
}
