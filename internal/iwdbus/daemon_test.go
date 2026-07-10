//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestDaemon_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("parseDaemonInfo", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name            string
			in              any
			want            DaemonInfo
			wantErrContains []string
		}{
			{
				name: "map[dbus.Variant]",
				in: map[string]dbus.Variant{
					"Version":                     dbus.MakeVariant("1.0.0"),
					"StateDirectory":              dbus.MakeVariant("/iwd/state"),
					"NetworkConfigurationEnabled": dbus.MakeVariant(true),
				},
				want: DaemonInfo{Version: "1.0.0", StateDirectory: "/iwd/state", NetworkConfigurationEnabled: true},
			},
			{
				name: "map[interface{}]",
				in: map[string]interface{}{
					"Version":                     dbus.MakeVariant("2.3.4"),
					"StateDirectory":              dbus.MakeVariant("/state"),
					"NetworkConfigurationEnabled": dbus.MakeVariant(false),
				},
				want: DaemonInfo{Version: "2.3.4", StateDirectory: "/state", NetworkConfigurationEnabled: false},
			},
			{
				name: "unexpected type",
				in:   12345,
				wantErrContains: []string{
					"dbus variant conversion error",
					"unexpected GetInfo payload type",
				},
			},
			{
				name: "empty map ok",
				in:   map[string]dbus.Variant{},
				want: DaemonInfo{},
			},
			{
				name: "extra keys ignored",
				in: map[string]dbus.Variant{
					"Version":                     dbus.MakeVariant("1.0"),
					"StateDirectory":              dbus.MakeVariant("/dir"),
					"NetworkConfigurationEnabled": dbus.MakeVariant(true),
					"NewField":                    dbus.MakeVariant("ignored"),
				},
				want: DaemonInfo{Version: "1.0", StateDirectory: "/dir", NetworkConfigurationEnabled: true},
			},
			{
				name: "wrong type - version",
				in: map[string]dbus.Variant{
					"Version":                     dbus.MakeVariant(123),
					"StateDirectory":              dbus.MakeVariant("/x"),
					"NetworkConfigurationEnabled": dbus.MakeVariant(true),
				},
				wantErrContains: []string{"dbus variant conversion error", "expected string"},
			},
			{
				name: "wrong type - state directory",
				in: map[string]dbus.Variant{
					"Version":                     dbus.MakeVariant("1"),
					"StateDirectory":              dbus.MakeVariant(false),
					"NetworkConfigurationEnabled": dbus.MakeVariant(true),
					"NewField":                    dbus.MakeVariant("ignored"),
				},
				wantErrContains: []string{"dbus variant conversion error", "expected string"},
			},
			{
				name: "wrong type - netconf",
				in: map[string]dbus.Variant{
					"Version":                     dbus.MakeVariant("1"),
					"StateDirectory":              dbus.MakeVariant("/x"),
					"NetworkConfigurationEnabled": dbus.MakeVariant("yes"),
				},
				wantErrContains: []string{"dbus variant conversion error", "expected bool"},
			},
			{
				name: "missing fields ok",
				in: map[string]dbus.Variant{
					"Version": dbus.MakeVariant("x"),
				},
				want: DaemonInfo{Version: "x"},
			},
			{
				name: "empty Version rejected",
				in: map[string]dbus.Variant{
					"Version": dbus.MakeVariant(""),
				},
				wantErrContains: []string{"dbus variant conversion error", "Version must not be empty"},
			},
			{
				name: "empty StateDirectory rejected",
				in: map[string]dbus.Variant{
					"Version":        dbus.MakeVariant("1"),
					"StateDirectory": dbus.MakeVariant(""),
				},
				wantErrContains: []string{"dbus variant conversion error", "StateDirectory must not be empty"},
			},
			{
				name: "map[interface{}] missing keys ok",
				in: map[string]interface{}{
					"Version": dbus.MakeVariant("7.0"),
				},
				want: DaemonInfo{Version: "7.0"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				info, err := parseDaemonInfo(tc.in)
				if len(tc.wantErrContains) > 0 {
					require.Error(t, err)
					for _, sub := range tc.wantErrContains {
						require.Contains(t, err.Error(), sub)
					}
					return
				}

				require.NoError(t, err)
				require.Equal(t, tc.want, *info)
			})
		}
	})

	t.Run("convenience", func(t *testing.T) {
		t.Run("GetVersion", func(t *testing.T) {
			t.Parallel()

			fake := &Daemon{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
				return []interface{}{map[string]interface{}{
					"Version":                     "9.9.9",
					"StateDirectory":              "/dir",
					"NetworkConfigurationEnabled": true,
				}}, nil
			}}}

			out, err := fake.GetVersion(context.Background())
			require.NoError(t, err, "failed to parse daemon info")
			require.Equal(t, "9.9.9", out)
		})

		t.Run("GetStateDirectory", func(t *testing.T) {
			t.Parallel()

			fake := &Daemon{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
				return []interface{}{map[string]interface{}{
					"Version":                     "1",
					"StateDirectory":              "/abc",
					"NetworkConfigurationEnabled": true,
				}}, nil
			}}}

			out, err := fake.GetStateDirectory(context.Background())
			require.NoError(t, err)
			require.Equal(t, "/abc", out)
		})

		t.Run("IsNetworkConfigurationEnabled", func(t *testing.T) {
			t.Parallel()

			fake := &Daemon{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
				return []interface{}{map[string]interface{}{
					"Version":                     "1",
					"StateDirectory":              "/def",
					"NetworkConfigurationEnabled": false,
				}}, nil
			}}}

			out, err := fake.IsNetworkConfigurationEnabled(context.Background())
			require.NoError(t, err)
			require.False(t, out)
		})
	})

	t.Run("GetInfo", func(t *testing.T) {
		t.Run("EmptyBody", func(t *testing.T) {
			t.Parallel()

			fake := &Daemon{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
				return []interface{}{}, nil
			}}}

			_, err := fake.GetInfo(context.Background())
			require.Error(t, err)
		})

		t.Run("NoIntro_ReturnsUninitialized", func(t *testing.T) {
			t.Parallel()

			d := &Daemon{intro: nil}
			_, err := d.GetInfo(context.Background())
			require.Error(t, err)
			require.ErrorIs(t, err, ErrDaemonUninitialized)
		})
	})

	t.Run("methods", func(t *testing.T) {
		t.Run("NoIntro_ReturnsUninitialized", func(t *testing.T) {
			t.Parallel()

			d := &Daemon{intro: nil}
			tests := []struct {
				name string
				fn   func() (any, error)
			}{
				{name: "GetInfo", fn: func() (any, error) { return d.GetInfo(context.Background()) }},
				{name: "GetVersion", fn: func() (any, error) { return d.GetVersion(context.Background()) }},
				{name: "GetStateDirectory", fn: func() (any, error) { return d.GetStateDirectory(context.Background()) }},
				{name: "IsNetworkConfigurationEnabled", fn: func() (any, error) { return d.IsNetworkConfigurationEnabled(context.Background()) }},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					_, err := tc.fn()
					require.Error(t, err)
					require.ErrorIs(t, err, ErrDaemonUninitialized)
				})
			}
		})

		t.Run("DBusError", func(t *testing.T) {
			t.Parallel()

			d := &Daemon{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
				return nil, fmt.Errorf("dbus failure")
			}}}

			tests := []struct {
				name string
				fn   func() (any, error)
			}{
				{name: "GetInfo", fn: func() (any, error) { return d.GetInfo(context.Background()) }},
				{name: "GetVersion", fn: func() (any, error) { return d.GetVersion(context.Background()) }},
				{name: "GetStateDirectory", fn: func() (any, error) { return d.GetStateDirectory(context.Background()) }},
				{name: "IsNetworkConfigurationEnabled", fn: func() (any, error) { return d.IsNetworkConfigurationEnabled(context.Background()) }},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					_, err := tc.fn()
					require.Error(t, err)
					require.Contains(t, err.Error(), "dbus failure")
				})
			}
		})

		t.Run("WrongTypePayload", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name            string
				payload         map[string]any
				call            func(d *Daemon) (any, error)
				wantErrContains []string
			}{
				{
					name: "GetInfo - wrong Version type",
					payload: map[string]any{
						"Version":                     123,
						"StateDirectory":              "/dir",
						"NetworkConfigurationEnabled": true,
					},
					call:            func(d *Daemon) (any, error) { return d.GetInfo(context.Background()) },
					wantErrContains: []string{"dbus variant conversion error", "expected string"},
				},
				{
					name: "GetVersion - wrong Version type",
					payload: map[string]any{
						"Version":                     123,
						"StateDirectory":              "/dir",
						"NetworkConfigurationEnabled": true,
					},
					call:            func(d *Daemon) (any, error) { return d.GetVersion(context.Background()) },
					wantErrContains: []string{"dbus variant conversion error", "expected string"},
				},
				{
					name: "GetStateDirectory - wrong StateDirectory type",
					payload: map[string]any{
						"Version":                     "1.0.0",
						"StateDirectory":              false,
						"NetworkConfigurationEnabled": true,
					},
					call:            func(d *Daemon) (any, error) { return d.GetStateDirectory(context.Background()) },
					wantErrContains: []string{"dbus variant conversion error", "expected string"},
				},
				{
					name: "IsNetworkConfigurationEnabled - wrong NetworkConfigurationEnabled type",
					payload: map[string]any{
						"Version":                     "1.0.0",
						"StateDirectory":              "/dev",
						"NetworkConfigurationEnabled": "yes",
					},
					call:            func(d *Daemon) (any, error) { return d.IsNetworkConfigurationEnabled(context.Background()) },
					wantErrContains: []string{"dbus variant conversion error", "expected bool"},
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					d := &Daemon{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
						return []interface{}{tc.payload}, nil
					}}}
					_, err := tc.call(d)
					require.Error(t, err)
					for _, sub := range tc.wantErrContains {
						require.Contains(t, err.Error(), sub)
					}
				})
			}
		})
	})
}

func TestDaemon_GetAdapters_Guards(t *testing.T) {
	t.Parallel()

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()

		var d *Daemon
		_, err := d.GetAdapters(context.Background())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDBusConnection)
		require.ErrorIs(t, err, ErrDaemonUninitialized)
	})

	t.Run("NilConn", func(t *testing.T) {
		t.Parallel()

		d := &Daemon{conn: nil}
		_, err := d.GetAdapters(context.Background())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDBusConnection)
		require.ErrorIs(t, err, ErrDaemonUninitialized)
	})
}

func TestDaemon_GetDevices_Guards(t *testing.T) {
	t.Parallel()

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()

		var d *Daemon
		_, err := d.GetDevices(context.Background())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDBusConnection)
		require.ErrorIs(t, err, ErrDaemonUninitialized)
	})

	t.Run("NilConn", func(t *testing.T) {
		t.Parallel()

		d := &Daemon{conn: nil}
		_, err := d.GetDevices(context.Background())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrDBusConnection)
		require.ErrorIs(t, err, ErrDaemonUninitialized)
	})
}

// TestDaemon_GetEnumerators_Guards covers the nil-receiver/nil-conn guards for
// the remaining enumerators (the enumeration bodies themselves need a real bus
// and are covered by the integration suite).
func TestDaemon_GetEnumerators_Guards(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		call func(d *Daemon) error
	}{
		{"GetBasicServiceSets", func(d *Daemon) error { _, err := d.GetBasicServiceSets(context.Background()); return err }},
		{"GetNetworks", func(d *Daemon) error { _, err := d.GetNetworks(context.Background()); return err }},
		{"GetKnownNetworks", func(d *Daemon) error { _, err := d.GetKnownNetworks(context.Background()); return err }},
		{"GetStations", func(d *Daemon) error { _, err := d.GetStations(context.Background()); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			t.Run("NilReceiver", func(t *testing.T) {
				t.Parallel()
				err := tc.call(nil)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrDBusConnection)
				require.ErrorIs(t, err, ErrDaemonUninitialized)
			})

			t.Run("NilConn", func(t *testing.T) {
				t.Parallel()
				err := tc.call(&Daemon{conn: nil})
				require.Error(t, err)
				require.ErrorIs(t, err, ErrDBusConnection)
				require.ErrorIs(t, err, ErrDaemonUninitialized)
			})
		})
	}
}

// TestBSSAddressFromManagedObject covers the address-extraction/validation helper
// used by GetBasicServiceSets. Its error branches are not reachable through the
// mock (which reports a valid Address), so they are exercised directly here.
func TestBSSAddressFromManagedObject(t *testing.T) {
	t.Parallel()

	const validPath = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/aabbccddeeff")

	tests := []struct {
		name            string
		path            dbus.ObjectPath
		props           map[string]dbus.Variant
		want            string
		wantErrContains []string
	}{
		{
			name:  "valid",
			path:  validPath,
			props: map[string]dbus.Variant{"Address": dbus.MakeVariant("aa:bb:cc:dd:ee:ff")},
			want:  "aa:bb:cc:dd:ee:ff",
		},
		{
			name:  "address is trimmed",
			path:  validPath,
			props: map[string]dbus.Variant{"Address": dbus.MakeVariant("  aa:bb:cc:dd:ee:ff  ")},
			want:  "aa:bb:cc:dd:ee:ff",
		},
		{
			name:            "invalid path",
			path:            dbus.ObjectPath(""),
			props:           map[string]dbus.Variant{"Address": dbus.MakeVariant("aa:bb:cc:dd:ee:ff")},
			wantErrContains: []string{"variant=Path", "invalid basic service set path"},
		},
		{
			name:            "missing Address property",
			path:            validPath,
			props:           map[string]dbus.Variant{},
			wantErrContains: []string{"variant=Address", "missing Address property"},
		},
		{
			name:            "Address wrong type",
			path:            validPath,
			props:           map[string]dbus.Variant{"Address": dbus.MakeVariant(42)},
			wantErrContains: []string{"variant=Address", "expected string, got int"},
		},
		{
			name:            "empty Address",
			path:            validPath,
			props:           map[string]dbus.Variant{"Address": dbus.MakeVariant("   ")},
			wantErrContains: []string{"variant=Address", "basic service set Address was empty"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := bssAddressFromManagedObject(tc.path, tc.props)
			if len(tc.wantErrContains) > 0 {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrDBusVariant)
				for _, sub := range tc.wantErrContains {
					require.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestStationNameFromDevice covers the best-effort station-name extraction from
// the co-located Device interface. Every non-happy branch returns "" rather than
// erroring, and the mock only exercises the happy path, so they are covered here.
func TestStationNameFromDevice(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		ifaces map[string]map[string]dbus.Variant
		want   string
	}{
		{
			name:   "resolves and trims device name",
			ifaces: map[string]map[string]dbus.Variant{IwdDeviceIface: {"Name": dbus.MakeVariant("  wlan0 ")}},
			want:   "wlan0",
		},
		{
			name:   "no device interface",
			ifaces: map[string]map[string]dbus.Variant{IwdStationIface: {}},
			want:   "",
		},
		{
			name:   "device interface without Name",
			ifaces: map[string]map[string]dbus.Variant{IwdDeviceIface: {"Powered": dbus.MakeVariant(true)}},
			want:   "",
		},
		{
			name:   "Name wrong type",
			ifaces: map[string]map[string]dbus.Variant{IwdDeviceIface: {"Name": dbus.MakeVariant(42)}},
			want:   "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, stationNameFromDevice(tc.ifaces))
		})
	}
}

// TestObjectNameFromManagedObject covers the name-extraction/validation helper
// shared by GetAdapters and GetDevices. Its error branches are not reachable
// through the mock (which always reports a valid Name), so they are exercised
// directly here.
func TestObjectNameFromManagedObject(t *testing.T) {
	t.Parallel()

	const validPath = dbus.ObjectPath("/net/connman/iwd/phy0")

	tests := []struct {
		name            string
		label           string
		path            dbus.ObjectPath
		props           map[string]dbus.Variant
		want            string
		wantErrContains []string
	}{
		{
			name:  "valid",
			label: "adapter",
			path:  validPath,
			props: map[string]dbus.Variant{"Name": dbus.MakeVariant("phy0")},
			want:  "phy0",
		},
		{
			name:  "name is trimmed",
			label: "device",
			path:  validPath,
			props: map[string]dbus.Variant{"Name": dbus.MakeVariant("  wlan0  ")},
			want:  "wlan0",
		},
		{
			name:            "invalid path",
			label:           "adapter",
			path:            dbus.ObjectPath(""),
			props:           map[string]dbus.Variant{"Name": dbus.MakeVariant("phy0")},
			wantErrContains: []string{"variant=Path", "invalid adapter path"},
		},
		{
			name:            "missing Name property",
			label:           "device",
			path:            validPath,
			props:           map[string]dbus.Variant{},
			wantErrContains: []string{"variant=Name", "missing Name property"},
		},
		{
			name:            "Name wrong type",
			label:           "adapter",
			path:            validPath,
			props:           map[string]dbus.Variant{"Name": dbus.MakeVariant(42)},
			wantErrContains: []string{"variant=Name", "expected string, got int"},
		},
		{
			name:            "empty Name",
			label:           "device",
			path:            validPath,
			props:           map[string]dbus.Variant{"Name": dbus.MakeVariant("")},
			wantErrContains: []string{"variant=Name", "device Name was empty"},
		},
		{
			name:            "whitespace Name",
			label:           "adapter",
			path:            validPath,
			props:           map[string]dbus.Variant{"Name": dbus.MakeVariant("   \t ")},
			wantErrContains: []string{"variant=Name", "adapter Name was empty"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := objectNameFromManagedObject(tc.label, tc.path, tc.props)
			if len(tc.wantErrContains) > 0 {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrDBusVariant)
				for _, sub := range tc.wantErrContains {
					require.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
