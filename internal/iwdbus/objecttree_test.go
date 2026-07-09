//go:build unit

package iwdbus

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func testTree() *ObjectTree {
	v := dbus.MakeVariant
	return &ObjectTree{objects: map[dbus.ObjectPath]map[string]map[string]dbus.Variant{
		"/net/connman/iwd/0/3": {
			IwdDeviceIface:  {"Name": v("wlan0")},
			IwdStationIface: {"Scanning": v(false)},
		},
		"/net/connman/iwd/0": {
			IwdAdapterIface: {"Name": v("phy0")},
		},
		"/net/connman/iwd/0/3/ssid_psk": {
			IwdNetworkIface: {"Name": v("ShadowGate")},
		},
		"/net/connman/iwd/0/3/ssid_psk/deadbeefcafe": {
			IwdBasicServiceSetIface: {"Address": v("de:ad:be:ef:ca:fe")},
		},
		"/net/connman/iwd/0/known": {
			IwdKnownNetworkIface: {"Name": v("HomeNet")},
		},
		"/net/connman/iwd/0/3/badtype": {
			IwdNetworkIface: {"Name": v(int32(7))}, // wrong type
		},
		"/net/connman/iwd/0/3/noname": {
			IwdNetworkIface: {"Connected": v(true)}, // iface present, Name property absent
		},
	}}
}

func TestObjectTree_Lookups(t *testing.T) {
	t.Parallel()
	tree := testTree()

	t.Run("hits", func(t *testing.T) {
		cases := []struct {
			got  func() (string, bool)
			want string
		}{
			{func() (string, bool) { return tree.DeviceName("/net/connman/iwd/0/3") }, "wlan0"},
			{func() (string, bool) { return tree.AdapterName("/net/connman/iwd/0") }, "phy0"},
			{func() (string, bool) { return tree.NetworkName("/net/connman/iwd/0/3/ssid_psk") }, "ShadowGate"},
			{func() (string, bool) {
				return tree.BSSAddress("/net/connman/iwd/0/3/ssid_psk/deadbeefcafe")
			}, "de:ad:be:ef:ca:fe"},
			{func() (string, bool) { return tree.KnownNetworkName("/net/connman/iwd/0/known") }, "HomeNet"},
		}
		for _, c := range cases {
			s, ok := c.got()
			require.True(t, ok)
			require.Equal(t, c.want, s)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		_, ok := tree.NetworkName("/nope")
		require.False(t, ok)
	})

	t.Run("wrong interface on path", func(t *testing.T) {
		// A device path is not a network.
		_, ok := tree.NetworkName("/net/connman/iwd/0/3")
		require.False(t, ok)
	})

	t.Run("wrong property type", func(t *testing.T) {
		_, ok := tree.NetworkName("/net/connman/iwd/0/3/badtype")
		require.False(t, ok)
	})

	t.Run("interface present but property absent", func(t *testing.T) {
		_, ok := tree.NetworkName("/net/connman/iwd/0/3/noname")
		require.False(t, ok)
	})

	t.Run("nil tree", func(t *testing.T) {
		var nilTree *ObjectTree
		_, ok := nilTree.DeviceName("/x")
		require.False(t, ok)
	})
}
