//go:build unit

package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

func TestAccessPoint_Core(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, NewAccessPoint(nil))
		require.NotNil(t, NewAccessPoint(&fakeAccessPointRaw{}))
	})

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()

		t.Run("Started", func(t *testing.T) {
			t.Parallel()
			started, err := NewAccessPoint(&fakeAccessPointRaw{started: true}).Started(ctx)
			require.NoError(t, err)
			require.True(t, started)
		})

		t.Run("OptionalsPassThrough", func(t *testing.T) {
			t.Parallel()
			name := "MyAP"
			freq := uint32(5180)
			group := "CCMP"
			a := NewAccessPoint(&fakeAccessPointRaw{
				name: &name, frequency: &freq, pairwiseCiphers: []string{"CCMP"}, groupCipher: &group,
			})
			gotName, err := a.Name(ctx)
			require.NoError(t, err)
			require.Equal(t, "MyAP", *gotName)
			gotFreq, err := a.Frequency(ctx)
			require.NoError(t, err)
			require.Equal(t, uint32(5180), *gotFreq)
			gotCiphers, err := a.PairwiseCiphers(ctx)
			require.NoError(t, err)
			require.Equal(t, []string{"CCMP"}, gotCiphers)
			gotGroup, err := a.GroupCipher(ctx)
			require.NoError(t, err)
			require.Equal(t, "CCMP", *gotGroup)
		})

		t.Run("ErrorClassifiedByResource", func(t *testing.T) {
			t.Parallel()
			_, err := NewAccessPoint(&fakeAccessPointRaw{err: iwdbus.ErrDBusMethod}).Scanning(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAccessPoint, ce.Resource)
		})
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			t.Parallel()
			name := "MyAP"
			a := NewAccessPoint(&fakeAccessPointRaw{props: &iwdbus.AccessPointProperties{
				Started: true, Scanning: false, Name: &name, PairwiseCiphers: []string{"CCMP"},
			}})
			props, err := a.Properties(ctx)
			require.NoError(t, err)
			require.True(t, props.Started)
			require.Equal(t, "MyAP", *props.Name)
			require.Equal(t, []string{"CCMP"}, props.PairwiseCiphers)
		})

		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			_, err := NewAccessPoint(&fakeAccessPointRaw{err: iwdbus.ErrDBusProperty}).Properties(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, ResourceAccessPoint, ce.Resource)
		})
	})

	t.Run("Start", func(t *testing.T) {
		t.Parallel()

		t.Run("ValidatesAndForwards", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{}
			require.NoError(t, NewAccessPoint(f).Start(ctx, "MyAP", "s3cretpass"))
			ssid, psk := f.startArgs()
			require.Equal(t, "MyAP", ssid)
			require.Equal(t, "s3cretpass", psk)
		})

		t.Run("InvalidFailsLocally", func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name string
				ssid string
				psk  string
			}{
				{"empty ssid", "", "12345678"},
				{"ssid too long", string(make([]byte, 33)), "12345678"},
				{"psk too short", "MyAP", "1234567"},
				{"psk too long", "MyAP", string(make([]byte, 64))},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := &fakeAccessPointRaw{}
					err := NewAccessPoint(f).Start(ctx, tc.ssid, tc.psk)
					require.Error(t, err)
					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, KindInvalidArgument, ce.Kind)
					require.Equal(t, ResourceAccessPoint, ce.Resource)
					ssid, psk := f.startArgs()
					require.Empty(t, ssid, "iwd must not be called for an invalid request")
					require.Empty(t, psk)
				})
			}
		})

		t.Run("BoundariesAccepted", func(t *testing.T) {
			t.Parallel()
			// The rejecting side is covered above. Pin the accepting side too, or
			// loosening `l > 32` to `l >= 32` (or `l < 8` to `l <= 8`) would pass the
			// whole suite: an SSID of 1 and 32 bytes, and a passphrase of 8 and 63
			// characters, are all valid and must reach iwd.
			for _, tc := range []struct {
				name string
				ssid string
				psk  string
			}{
				{"shortest ssid", "a", "12345678"},
				{"longest ssid", strings.Repeat("a", 32), "12345678"},
				{"shortest psk", "MyAP", strings.Repeat("p", 8)},
				{"longest psk", "MyAP", strings.Repeat("p", 63)},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := &fakeAccessPointRaw{}
					require.NoError(t, NewAccessPoint(f).Start(ctx, tc.ssid, tc.psk))
					ssid, psk := f.startArgs()
					require.Equal(t, tc.ssid, ssid, "a boundary-valid request must reach iwd")
					require.Equal(t, tc.psk, psk)
				})
			}
		})

		t.Run("AlreadyExistsSentinelPreserved", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{err: fmt.Errorf("%w: %w", iwdbus.ErrDBusMethod, iwdbus.ErrAlreadyExists)}
			err := NewAccessPoint(f).Start(ctx, "MyAP", "s3cretpass")
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrAlreadyExists)
		})
	})

	t.Run("StartProfile", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{}
			require.NoError(t, NewAccessPoint(f).StartProfile(ctx, "HomeAP"))
			require.Equal(t, "HomeAP", f.profileArg())
		})

		t.Run("InvalidSSIDFailsLocally", func(t *testing.T) {
			t.Parallel()
			// StartProfile validates the SSID with the same 1-32 byte rule as Start,
			// so both rejecting bounds must fail before any D-Bus round-trip.
			for _, tc := range []struct {
				name string
				ssid string
			}{
				{"empty ssid", ""},
				{"ssid too long", strings.Repeat("a", 33)},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := &fakeAccessPointRaw{}
					err := NewAccessPoint(f).StartProfile(ctx, tc.ssid)
					require.Error(t, err)
					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, KindInvalidArgument, ce.Kind)
					require.Equal(t, ResourceAccessPoint, ce.Resource)
					require.Empty(t, f.profileArg(), "iwd must not be called for an invalid request")
				})
			}
		})

		t.Run("BoundarySSIDsAccepted", func(t *testing.T) {
			t.Parallel()
			for _, ssid := range []string{"a", strings.Repeat("a", 32)} {
				t.Run(fmt.Sprintf("len%d", len(ssid)), func(t *testing.T) {
					t.Parallel()
					f := &fakeAccessPointRaw{}
					require.NoError(t, NewAccessPoint(f).StartProfile(ctx, ssid))
					require.Equal(t, ssid, f.profileArg())
				})
			}
		})
	})

	t.Run("StopScan", func(t *testing.T) {
		t.Parallel()

		t.Run("Stop", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{}
			require.NoError(t, NewAccessPoint(f).Stop(ctx))
			require.True(t, f.stopWasCalled())
		})

		t.Run("Scan", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{}
			require.NoError(t, NewAccessPoint(f).Scan(ctx))
			require.True(t, f.scanWasCalled())
		})

		t.Run("ScanError", func(t *testing.T) {
			t.Parallel()
			err := NewAccessPoint(&fakeAccessPointRaw{err: iwdbus.ErrDBusMethod}).Scan(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, ResourceAccessPoint, ce.Resource)
		})
	})

	t.Run("OrderedNetworks", func(t *testing.T) {
		t.Parallel()

		t.Run("Converts", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{ordered: []iwdbus.AccessPointOrderedNetwork{
				{Name: "OpenNet", SignalStrength: -6000, Type: iwdbus.NetworkTypeOpen},
				{Name: "SecuredNet", SignalStrength: -7200, Type: iwdbus.NetworkTypePSK},
			}}
			got, err := NewAccessPoint(f).OrderedNetworks(ctx)
			require.NoError(t, err)
			require.Equal(t, []AccessPointOrderedNetwork{
				{Name: "OpenNet", SignalStrength: -6000, Type: NetworkTypeOpen},
				{Name: "SecuredNet", SignalStrength: -7200, Type: NetworkTypePSK},
			}, got)
		})

		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			_, err := NewAccessPoint(&fakeAccessPointRaw{err: iwdbus.ErrDBusMethod}).OrderedNetworks(ctx)
			require.Error(t, err)
		})
	})

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()

		t.Run("PropertiesChangedNormalizes", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{subEvent: &iwdbus.AccessPointPropertiesChanged{
				Changed:     map[string]dbus.Variant{"Started": dbus.MakeVariant(true)},
				Invalidated: []string{"Name"},
			}}
			var got AccessPointPropertiesChanged
			_, err := NewAccessPoint(f).SubscribePropertiesChanged(ctx, func(ev AccessPointPropertiesChanged) { got = ev })
			require.NoError(t, err)
			require.Equal(t, true, got.Changed["Started"])
			require.Equal(t, []string{"Name"}, got.Invalidated)
		})

		t.Run("StartedChanged", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{subEvent: &iwdbus.AccessPointPropertiesChanged{
				Changed: map[string]dbus.Variant{"Started": dbus.MakeVariant(false)},
			}}
			var got bool
			var fired bool
			_, err := NewAccessPoint(f).SubscribeStartedChanged(ctx, func(b bool) { got = b; fired = true })
			require.NoError(t, err)
			require.True(t, fired)
			require.False(t, got)
		})

		t.Run("ScanningChanged", func(t *testing.T) {
			t.Parallel()
			f := &fakeAccessPointRaw{subEvent: &iwdbus.AccessPointPropertiesChanged{
				Changed: map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)},
			}}
			var got bool
			var fired bool
			_, err := NewAccessPoint(f).SubscribeScanningChanged(ctx, func(b bool) { got = b; fired = true })
			require.NoError(t, err)
			require.True(t, fired)
			require.True(t, got)
		})

		t.Run("NilCallbackRejected", func(t *testing.T) {
			t.Parallel()
			_, err := NewAccessPoint(&fakeAccessPointRaw{}).SubscribeScanningChanged(ctx, nil)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrCore)
		})

		t.Run("BackendErrors", func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name string
				call func(*AccessPoint) error
			}{
				{"PropertiesChanged", func(a *AccessPoint) error {
					_, err := a.SubscribePropertiesChanged(ctx, func(AccessPointPropertiesChanged) {})
					return err
				}},
				{"StartedChanged", func(a *AccessPoint) error {
					_, err := a.SubscribeStartedChanged(ctx, func(bool) {})
					return err
				}},
				{"ScanningChanged", func(a *AccessPoint) error {
					_, err := a.SubscribeScanningChanged(ctx, func(bool) {})
					return err
				}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					a := NewAccessPoint(&fakeAccessPointRaw{err: iwdbus.ErrDBusMethod})
					err := tc.call(a)
					require.Error(t, err)
					require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, ResourceAccessPoint, ce.Resource)
				})
			}
		})
	})

	t.Run("NotInitialized", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name string
			call func(*AccessPoint) error
		}{
			{"Started", func(a *AccessPoint) error { _, err := a.Started(ctx); return err }},
			{"Scanning", func(a *AccessPoint) error { _, err := a.Scanning(ctx); return err }},
			{"Name", func(a *AccessPoint) error { _, err := a.Name(ctx); return err }},
			{"Frequency", func(a *AccessPoint) error { _, err := a.Frequency(ctx); return err }},
			{"PairwiseCiphers", func(a *AccessPoint) error { _, err := a.PairwiseCiphers(ctx); return err }},
			{"GroupCipher", func(a *AccessPoint) error { _, err := a.GroupCipher(ctx); return err }},
			{"Properties", func(a *AccessPoint) error { _, err := a.Properties(ctx); return err }},
			{"Start", func(a *AccessPoint) error { return a.Start(ctx, "MyAP", "12345678") }},
			{"StartProfile", func(a *AccessPoint) error { return a.StartProfile(ctx, "x") }},
			{"Stop", func(a *AccessPoint) error { return a.Stop(ctx) }},
			{"Scan", func(a *AccessPoint) error { return a.Scan(ctx) }},
			{"OrderedNetworks", func(a *AccessPoint) error { _, err := a.OrderedNetworks(ctx); return err }},
			{"SubscribePropertiesChanged", func(a *AccessPoint) error {
				_, err := a.SubscribePropertiesChanged(ctx, func(AccessPointPropertiesChanged) {})
				return err
			}},
			{"SubscribeStartedChanged", func(a *AccessPoint) error {
				_, err := a.SubscribeStartedChanged(ctx, func(bool) {})
				return err
			}},
			{"SubscribeScanningChanged", func(a *AccessPoint) error {
				_, err := a.SubscribeScanningChanged(ctx, func(bool) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				var a *AccessPoint
				err := tc.call(a)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrAccessPointNotInitialized)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, KindInvalidState, ce.Kind)
				require.Equal(t, ResourceAccessPoint, ce.Resource)
			})
		}
	})
}
