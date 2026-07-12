//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestAccessPoint_Public(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Identity", func(t *testing.T) {
		t.Parallel()
		ap := newAccessPoint(&fakeCoreAccessPoint{}, "/net/connman/iwd/0/4", "wlan1")
		require.Equal(t, "/net/connman/iwd/0/4", ap.Path())
		require.Equal(t, "wlan1", ap.Name())
	})

	t.Run("NameVsSSID", func(t *testing.T) {
		t.Parallel()
		ssid := "MyHostedNet"
		ap := newAccessPoint(&fakeCoreAccessPoint{ssid: &ssid}, "/p", "wlan1")
		require.Equal(t, "wlan1", ap.Name(), "Name is the device name")
		gotSSID, err := ap.SSID(ctx)
		require.NoError(t, err)
		require.Equal(t, "MyHostedNet", *gotSSID, "SSID is the hosted network name")
	})

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()
		freq := uint32(5180)
		group := "CCMP"
		ap := newAccessPoint(&fakeCoreAccessPoint{
			started: true, frequency: &freq, pairwiseCiphers: []string{"CCMP"}, groupCipher: &group,
		}, "/p", "wlan1")

		started, err := ap.Started(ctx)
		require.NoError(t, err)
		require.True(t, started)
		f, err := ap.Frequency(ctx)
		require.NoError(t, err)
		require.Equal(t, uint32(5180), *f)
		pc, err := ap.PairwiseCiphers(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{"CCMP"}, pc)
		gc, err := ap.GroupCipher(ctx)
		require.NoError(t, err)
		require.Equal(t, "CCMP", *gc)
	})

	t.Run("PropertiesRenamesNameToSSID", func(t *testing.T) {
		t.Parallel()
		ssid := "MyHostedNet"
		ap := newAccessPoint(&fakeCoreAccessPoint{props: &core.AccessPointProperties{
			Started: true, Name: &ssid, PairwiseCiphers: []string{"CCMP"},
		}}, "/p", "wlan1")
		props, err := ap.Properties(ctx)
		require.NoError(t, err)
		require.True(t, props.Started)
		require.NotNil(t, props.SSID)
		require.Equal(t, "MyHostedNet", *props.SSID)
		require.Equal(t, []string{"CCMP"}, props.PairwiseCiphers)
	})

	t.Run("PropertiesError", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("boom")
		_, err := newAccessPoint(&fakeCoreAccessPoint{err: wantErr}, "/p", "wlan1").Properties(ctx)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("StartDelegates", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{}
		require.NoError(t, newAccessPoint(f, "/p", "wlan1").Start(ctx, "MyAP", "s3cretpass"))
		ssid, psk := f.startArgs()
		require.Equal(t, "MyAP", ssid)
		require.Equal(t, "s3cretpass", psk)
	})

	t.Run("StartError", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("already exists")
		f := &fakeCoreAccessPoint{err: wantErr}
		require.ErrorIs(t, newAccessPoint(f, "/p", "wlan1").Start(ctx, "MyAP", "s3cretpass"), wantErr)
	})

	t.Run("Scanning", func(t *testing.T) {
		t.Parallel()
		s, err := newAccessPoint(&fakeCoreAccessPoint{scanning: true}, "/p", "wlan1").Scanning(ctx)
		require.NoError(t, err)
		require.True(t, s)
	})

	t.Run("StartProfileDelegates", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{}
		require.NoError(t, newAccessPoint(f, "/p", "wlan1").StartProfile(ctx, "HomeAP"))
		require.Equal(t, []string{"StartProfile"}, f.callList())
		require.Equal(t, "HomeAP", f.profileArg())
	})

	t.Run("StartProfileError", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("not found")
		f := &fakeCoreAccessPoint{err: wantErr}
		require.ErrorIs(t, newAccessPoint(f, "/p", "wlan1").StartProfile(ctx, "HomeAP"), wantErr)
	})

	t.Run("StopScanDelegate", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{}
		ap := newAccessPoint(f, "/p", "wlan1")
		require.NoError(t, ap.Stop(ctx))
		require.NoError(t, ap.Scan(ctx))
		require.Equal(t, []string{"Stop", "Scan"}, f.callList())
	})

	t.Run("OrderedNetworksConverts", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{ordered: []core.AccessPointOrderedNetwork{
			{Name: "OpenNet", SignalStrength: -6000, Type: core.NetworkTypeOpen},
			{Name: "SecuredNet", SignalStrength: -7200, Type: core.NetworkTypePSK},
		}}
		got, err := newAccessPoint(f, "/p", "wlan1").OrderedNetworks(ctx)
		require.NoError(t, err)
		require.Equal(t, []AccessPointOrderedNetwork{
			{Name: "OpenNet", SignalStrength: -60, Type: NetworkTypeOpen},
			{Name: "SecuredNet", SignalStrength: -72, Type: NetworkTypePSK},
		}, got)
	})

	t.Run("OrderedNetworksToleratesUnknownType", func(t *testing.T) {
		t.Parallel()
		// A neighbor whose security iwd cannot classify (empty/unrecognized) must
		// surface as NetworkTypeUnknown, not fail the whole list.
		f := &fakeCoreAccessPoint{ordered: []core.AccessPointOrderedNetwork{
			{Name: "OpenNet", SignalStrength: -6000, Type: core.NetworkTypeOpen},
			{Name: "MysteryNet", SignalStrength: -8100, Type: core.NetworkType("")},
		}}
		got, err := newAccessPoint(f, "/p", "wlan1").OrderedNetworks(ctx)
		require.NoError(t, err)
		require.Equal(t, []AccessPointOrderedNetwork{
			{Name: "OpenNet", SignalStrength: -60, Type: NetworkTypeOpen},
			{Name: "MysteryNet", SignalStrength: -81, Type: NetworkTypeUnknown},
		}, got)
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{subEvent: &core.AccessPointPropertiesChanged{
			Changed: map[string]any{"Started": true}, Invalidated: []string{"Name"},
		}}
		var got AccessPointPropertiesChanged
		_, err := newAccessPoint(f, "/p", "wlan1").SubscribePropertiesChanged(ctx, func(ev AccessPointPropertiesChanged) { got = ev })
		require.NoError(t, err)
		require.Equal(t, true, got.Changed["Started"])
		require.Equal(t, []string{"Name"}, got.Invalidated)
	})

	t.Run("SubscribeStartedChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{subEvent: &core.AccessPointPropertiesChanged{
			Changed: map[string]any{"Started": true},
		}}
		var got, fired bool
		_, err := newAccessPoint(f, "/p", "wlan1").SubscribeStartedChanged(ctx, func(b bool) { got = b; fired = true })
		require.NoError(t, err)
		require.True(t, fired)
		require.True(t, got)
	})

	t.Run("SubscribeScanningChanged", func(t *testing.T) {
		t.Parallel()
		f := &fakeCoreAccessPoint{subEvent: &core.AccessPointPropertiesChanged{
			Changed: map[string]any{"Scanning": true},
		}}
		var got, fired bool
		_, err := newAccessPoint(f, "/p", "wlan1").SubscribeScanningChanged(ctx, func(b bool) { got = b; fired = true })
		require.NoError(t, err)
		require.True(t, fired)
		require.True(t, got)
	})

	t.Run("SubscribeNilCallbackRejected", func(t *testing.T) {
		t.Parallel()
		_, err := newAccessPoint(&fakeCoreAccessPoint{}, "/p", "wlan1").SubscribeStartedChanged(ctx, nil)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("SubscribeBackendErrors", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("subscribe boom")
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
				a := newAccessPoint(&fakeCoreAccessPoint{err: wantErr}, "/p", "wlan1")
				require.ErrorIs(t, tc.call(a), wantErr)
			})
		}
	})

	t.Run("NilAccessPoint", func(t *testing.T) {
		t.Parallel()
		var ap *AccessPoint
		require.Empty(t, ap.Path())
		require.Empty(t, ap.Name())
		for _, tc := range []struct {
			name string
			call func(*AccessPoint) error
		}{
			{"Started", func(a *AccessPoint) error { _, err := a.Started(ctx); return err }},
			{"Scanning", func(a *AccessPoint) error { _, err := a.Scanning(ctx); return err }},
			{"SSID", func(a *AccessPoint) error { _, err := a.SSID(ctx); return err }},
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
				require.ErrorIs(t, tc.call(ap), ErrInternal)
			})
		}
	})
}
