//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// apDevicePath is the object path of the mock AP-mode device (wlan1, on the
// second adapter). The AccessPoint interface is exported on it.
const apDevicePath = "/net/connman/iwd/1/4"

// TestAccessPointMock_ReadProperties reads the running AP's properties end to end
// over the real session bus: Started plus the optional hosted-network fields.
func TestAccessPointMock_ReadProperties(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)
	require.Equal(t, "wlan1", ap.Name())

	props, err := ap.Properties(ctx)
	require.NoError(t, err)
	require.True(t, props.Started)
	require.False(t, props.Scanning)
	require.NotNil(t, props.SSID)
	require.Equal(t, "MockAP", *props.SSID)
	require.NotNil(t, props.Frequency)
	require.Equal(t, uint32(5180), *props.Frequency)
	require.Equal(t, []string{"CCMP"}, props.PairwiseCiphers)
	require.NotNil(t, props.GroupCipher)
	require.Equal(t, "CCMP", *props.GroupCipher)
}

// TestAccessPointMock_AllAccessPoints verifies access-point enumeration via the
// real ObjectManager returns exactly the AP-mode device (wlan1), not the
// station-mode device (wlan0).
func TestAccessPointMock_AllAccessPoints(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	aps, err := client.AllAccessPoints(ctx)
	require.NoError(t, err)
	require.Len(t, aps, 1)
	require.Equal(t, apDevicePath, aps[0].Path())
	require.Equal(t, "wlan1", aps[0].Name())

	// The enumerated handle is live.
	started, err := aps[0].Started(ctx)
	require.NoError(t, err)
	require.True(t, started)
}

// TestAccessPointMock_StartAlreadyExists verifies iwd's AlreadyExists taxonomy
// surfaces end to end: the mock AP is already running, so Start is rejected with
// the matchable public sentinel.
func TestAccessPointMock_StartAlreadyExists(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)

	err = ap.Start(ctx, "OtherAP", "s3cretpass")
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrAlreadyExists)
}

// TestAccessPointMock_StopThenStart stops the running AP and starts a fresh one,
// confirming both operations mutate Started over the real bus.
func TestAccessPointMock_StopThenStart(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)

	require.NoError(t, ap.Stop(ctx))
	started, err := ap.Started(ctx)
	require.NoError(t, err)
	require.False(t, started)

	require.NoError(t, ap.Start(ctx, "FreshAP", "s3cretpass"))
	started, err = ap.Started(ctx)
	require.NoError(t, err)
	require.True(t, started)

	ssid, err := ap.SSID(ctx)
	require.NoError(t, err)
	require.NotNil(t, ssid)
	require.Equal(t, "FreshAP", *ssid)
}

// TestAccessPointMock_PropertiesWhenStopped is the regression guard for the
// hardware bug where reading a stopped AP failed: iwd omits Scanning (and the
// other optional properties) while the AP is not running, so Properties must
// still succeed and report Scanning as false rather than erroring on the absent
// property.
func TestAccessPointMock_PropertiesWhenStopped(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)
	require.NoError(t, ap.Stop(ctx))

	props, err := ap.Properties(ctx)
	require.NoError(t, err)
	require.False(t, props.Started)
	require.False(t, props.Scanning)
	require.Nil(t, props.SSID)
	require.Nil(t, props.Frequency)

	// The standalone Scanning getter tolerates the absent property too.
	scanning, err := ap.Scanning(ctx)
	require.NoError(t, err)
	require.False(t, scanning)

	// The `access-point status` CLI path (Properties for every AP) renders cleanly,
	// showing only Started for the stopped AP (no Scanning line).
	out, err := runSpider(t, "access-point", "status")
	require.NoError(t, err, out)
	mustContain(t, out, "wlan1")
	mustContain(t, out, "Started:  false")
	require.NotContains(t, out, "Scanning", "stopped AP must not print a Scanning line:\n%s", out)
}

// TestAccessPointMock_StartProfile starts from the mock's stored profile after a
// stop, and confirms an unknown profile is rejected NotFound.
func TestAccessPointMock_StartProfile(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)
	require.NoError(t, ap.Stop(ctx))

	err = ap.StartProfile(ctx, "NoSuchProfile")
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrNotFound)

	require.NoError(t, ap.StartProfile(ctx, "MockProfile"))
	started, err := ap.Started(ctx)
	require.NoError(t, err)
	require.True(t, started)
}

// TestAccessPointMock_ScanLiveTransition drives a real AP scan and observes the
// Scanning property transition true->false over live PropertiesChanged signals.
func TestAccessPointMock_ScanLiveTransition(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)

	scanningTrue := make(chan struct{}, 1)
	scanningFalse := make(chan struct{}, 1)
	unsub, err := ap.SubscribeScanningChanged(ctx, func(scanning bool) {
		if scanning {
			select {
			case scanningTrue <- struct{}{}:
			default:
			}
			return
		}
		select {
		case scanningFalse <- struct{}{}:
		default:
		}
	})
	require.NoError(t, err)
	defer func() { _ = unsub.Unsubscribe() }()

	require.NoError(t, ap.Scan(ctx))
	requireFired(t, scanningTrue, "expected Scanning true")
	requireFired(t, scanningFalse, "expected Scanning false")
}

// TestAccessPointMock_OrderedNetworks reads the seeded AP scan result end to end,
// confirming the aa{sv} reply parses into resolved SSIDs, dBm signals, and types.
// The final entry has an unclassifiable security, exercising the tolerant path
// that surfaces it as an unknown type rather than failing the whole list (the
// hardware bug where a single unknown neighbor broke `access-point networks`).
func TestAccessPointMock_OrderedNetworks(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.NoError(t, err)

	nets, err := ap.OrderedNetworks(ctx)
	require.NoError(t, err)
	require.Equal(t, []spiderw.AccessPointOrderedNetwork{
		{Name: "OpenNet", SignalStrength: -60, Type: spiderw.NetworkTypeOpen},
		{Name: "SecuredNet", SignalStrength: -72, Type: spiderw.NetworkTypePSK},
		{Name: "MysteryNet", SignalStrength: -81, Type: spiderw.NetworkTypeUnknown},
	}, nets)
}

// TestAccessPointMock_Unavailable verifies that when the AccessPoint interface is
// not exported (--omit-access-point), Client.AccessPoint fails cleanly and
// enumeration is empty, even though the AP-mode device object still exists.
func TestAccessPointMock_Unavailable(t *testing.T) {
	iwdmock.StartMockWithoutAccessPoint(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	ap, err := client.AccessPoint(ctx, apDevicePath)
	require.Error(t, err)
	require.Nil(t, ap)

	aps, err := client.AllAccessPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, aps)

	// The device itself is still present, confirming only the AccessPoint
	// interface was omitted.
	device, err := client.Device(ctx, apDevicePath)
	require.NoError(t, err)
	name, err := device.Name(ctx)
	require.NoError(t, err)
	require.Equal(t, "wlan1", name)
}

// TestAccessPointMock_CLI_Status drives `access-point status` in-process against
// the mock, confirming the read path renders end to end.
func TestAccessPointMock_CLI_Status(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "access-point", "status")
	require.NoError(t, err, out)
	mustContainAll(t, out, []string{"wlan1", apDevicePath, "MockAP", "5180", "CCMP"})
}

// TestAccessPointMock_CLI_List drives `access-point list` against the mock.
func TestAccessPointMock_CLI_List(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "access-point", "list")
	require.NoError(t, err, out)
	mustContainAll(t, out, []string{"wlan1", apDevicePath})
}

// TestAccessPointMock_CLI_ScanWaits drives `access-point <ap> scan` in wait mode
// (the default), confirming it blocks for the live Scanning true->false
// transition and then lists the scan results, mirroring `station scan`.
func TestAccessPointMock_CLI_ScanWaits(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "access-point", "wlan1", "scan")
	require.NoError(t, err, out)
	mustContainAll(t, out, []string{"scan started", "OpenNet", "SecuredNet"})
}

// TestAccessPointMock_CLI_ScanNoWait drives `access-point <ap> scan --no-wait`,
// which returns immediately without listing results.
func TestAccessPointMock_CLI_ScanNoWait(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "access-point", "wlan1", "scan", "--no-wait")
	require.NoError(t, err, out)
	mustContain(t, out, "scan started")
}

// TestAccessPointMock_CLI_Networks drives `access-point <ap> networks`, confirming
// the seeded scan result renders with SSIDs and dBm signals.
func TestAccessPointMock_CLI_Networks(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "access-point", "wlan1", "networks")
	require.NoError(t, err, out)
	// Includes MysteryNet, whose unclassified security renders as "unknown".
	mustContainAll(t, out, []string{"OpenNet", "SecuredNet", "MysteryNet", "-60", "-72", "unknown"})
}
