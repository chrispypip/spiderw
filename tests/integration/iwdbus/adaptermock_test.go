//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// These tests exercise the adapter against a real D-Bus round trip. Per the
// integration testing convention, the public Go API is the baseline (it is the
// primary product surface and carries typed values/errors); the CLI gets a thin
// layer covering only CLI-specific behavior (command routing, output rendering,
// argument validation, exit codes); and raw iwdbus tests cover signal plumbing
// that lives at that layer. Exhaustive per-property / per-mode value and error
// matrices live in the iwdbus, core, and public unit tests and are not re-tested
// here.

func newTestAdapter(t *testing.T) (*iwdbus.Adapter, *dbus.Conn) {
	t.Helper()

	conn, err := dbus.SessionBus()
	require.NoError(t, err)

	adapter, err := iwdbus.NewAdapter(
		context.Background(),
		conn,
		"/net/connman/iwd/phy0",
	)
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })

	return adapter, conn
}

func newPublicMockAdapter(t *testing.T, ctx context.Context, client *spiderw.Client, name string) *spiderw.Adapter {
	t.Helper()

	daemon := client.Daemon()
	require.NotNil(t, daemon)

	refs, err := daemon.Adapters(ctx)
	require.NoError(t, err)

	for _, ref := range refs {
		if ref.Name != name {
			continue
		}

		adapter, err := client.Adapter(ctx, ref.Path)
		require.NoError(t, err)
		require.NotNil(t, adapter)
		return adapter
	}

	t.Fatalf("mock adapter %q not found in refs: %#v", name, refs)
	return nil
}

func drainBoolChan(ch <-chan bool) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func runSpiderAdapter(t *testing.T, args ...string) (string, error) {
	t.Helper()

	return runSpider(t, append([]string{"adapter"}, args...)...)
}

func runSpiderAdapterJSON(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()

	return runSpiderJSON(t, append([]string{"adapter"}, args...)...)
}

// -----------------------------------------------------------------------------
// Public API against the mock
// -----------------------------------------------------------------------------

// TestAdapterMock_Properties exercises the batched Properties (Properties.GetAll)
// path through the public API against the iwd mock. It is the baseline coverage
// for every per-property getter (Powered/Name/Model/Vendor/SupportedModes).
func TestAdapterMock_Properties(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	a := newPublicMockAdapter(t, ctx, newMockClient(t, ctx), "phy0")

	props, err := a.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, "phy0", props.Name)
	require.True(t, props.Powered)
	require.NotNil(t, props.Model)
	require.Equal(t, "MockModel", *props.Model)
	require.NotNil(t, props.Vendor)
	require.Equal(t, "MockVendor", *props.Vendor)
	require.ElementsMatch(t, []spiderw.Mode{spiderw.ModeStation, spiderw.ModeAP}, props.SupportedModes)
}

// TestAdapterMock_PropertiesOmittedOptionals confirms an absent optional is
// simply missing from the GetAll reply and stays nil without erroring.
func TestAdapterMock_PropertiesOmittedOptionals(t *testing.T) {
	iwdmock.StartMockWithOmittedOptionals(t)

	ctx := context.Background()
	a := newPublicMockAdapter(t, ctx, newMockClient(t, ctx), "phy0")

	props, err := a.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, "phy0", props.Name)
	require.Nil(t, props.Model)
	require.Nil(t, props.Vendor)
	require.ElementsMatch(t, []spiderw.Mode{spiderw.ModeStation, spiderw.ModeAP}, props.SupportedModes)
}

// TestAdapterMock_SupportsMode covers the mode-support queries through the public
// API; the mock advertises station + ap.
func TestAdapterMock_SupportsMode(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	a := newPublicMockAdapter(t, ctx, newMockClient(t, ctx), "phy0")

	station, err := a.SupportsStation(ctx)
	require.NoError(t, err)
	require.True(t, station)

	ap, err := a.SupportsMode(ctx, spiderw.ModeAP)
	require.NoError(t, err)
	require.True(t, ap)

	adhoc, err := a.SupportsAdHoc(ctx)
	require.NoError(t, err)
	require.False(t, adhoc)
}

func TestAdapterMock_SubscribePoweredChanged_UnsubscribeStopsCallbacks(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	adapter := newPublicMockAdapter(t, ctx, newMockClient(t, ctx), "phy0")
	received := make(chan bool, 2)

	unsubscribe, err := adapter.SubscribePoweredChanged(ctx, func(powered bool) {
		received <- powered
	})
	require.NoError(t, err)

	require.NoError(t, adapter.SetPowered(ctx, false))
	select {
	case got := <-received:
		require.False(t, got)
	case <-time.After(signalTimeout):
		t.Fatal("timed out waiting for powered=false callback")
	}

	require.NoError(t, unsubscribe.Unsubscribe())
	require.NoError(t, unsubscribe.Unsubscribe(), "unsubscribe should be idempotent")
	drainBoolChan(received)

	require.NoError(t, adapter.SetPowered(ctx, true))
	require.Never(t, func() bool {
		select {
		case got := <-received:
			t.Logf("unexpected callback after unsubscribe: powered=%t", got)
			return true
		default:
			return false
		}
	}, 250*time.Millisecond, pollInterval, "callback fired after unsubscribe")
}

// -----------------------------------------------------------------------------
// Raw iwdbus signal plumbing against the mock
// -----------------------------------------------------------------------------

func TestAdapterMock_SubscribePropertiesChanged(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	adapter, _ := newTestAdapter(t)

	var recv iwdbus.AdapterPropertiesChanged
	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePropertiesChanged(context.Background(), func(changed iwdbus.AdapterPropertiesChanged) {
		recv = changed
		fired <- struct{}{}
	})
	require.NoError(t, err)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "powered", "false")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Powered"))

	requireFired(t, fired)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(false),
	}
	require.Len(t, changed, 1)
	require.Equal(t, changed, recv.Changed)
}

func TestAdapterMock_SubscribePoweredChanged(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	adapter, _ := newTestAdapter(t)

	var recv bool
	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePoweredChanged(context.Background(), func(v bool) {
		recv = v
		fired <- struct{}{}
	})
	require.NoError(t, err)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "powered", "disable")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Powered"))

	requireFired(t, fired)

	require.Equal(t, false, recv)
}

func TestAdapterMock_Firehose(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	adapter, _ := newTestAdapter(t)

	var recv iwdbus.FirehoseSignal
	fired := make(chan struct{}, 1)

	err := adapter.Firehose(context.Background(), func(sig iwdbus.FirehoseSignal) {
		recv = sig
		fired <- struct{}{}
	})
	require.NoError(t, err)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "powered", "disabled")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Powered"))

	changed := map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)}

	requireFired(t, fired)

	require.Equal(t, "org.freedesktop.DBus.Properties", recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
	require.Len(t, recv.Body, 3)

	s, ok := recv.Body[0].(string)
	require.True(t, ok)

	v, ok := recv.Body[1].(map[string]dbus.Variant)
	require.True(t, ok)

	ss, ok := recv.Body[2].([]string)
	require.True(t, ok)

	require.Equal(t, iwdbus.IwdAdapterIface, s)
	require.Equal(t, changed, v)
	require.Equal(t, []string{}, ss)
}

// -----------------------------------------------------------------------------
// CLI (`spiderw adapter …`) against the mock — thin, CLI-specific coverage only
// -----------------------------------------------------------------------------

// TestAdapterMock_Status exercises `adapter status`, which drives
// Client.AllAdapters: it constructs a handle per adapter and reports the full
// per-adapter snapshot (path, name, powered, model, vendor, supported modes).
func TestAdapterMock_Status(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderAdapter(t, "status")
	require.NoError(t, err, "output:\n%s", out)

	// Human output is an aligned key:value block per adapter.
	mustContainAll(t, out, []string{
		"Name:",
		"phy0",
		"Path:",
		"/net/connman/iwd/phy0",
		"Powered:",
		"true",
		"Model:",
		"MockModel",
		"Vendor:",
		"MockVendor",
		"SupportedModes:",
		"station",
		"ap",
	})
}

// TestAdapterMock_ScopedStatusJSON exercises `adapter <adapter> status --json`
// through the real D-Bus stack and verifies it keeps the same one-entry array
// shape as aggregate `adapter status --json`.
func TestAdapterMock_ScopedStatusJSON(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	list, out, err := runSpiderJSONArray(t, "adapter", "phy0", "status")
	require.NoError(t, err, "output:\n%s", out)
	require.Len(t, list, 1, "output:\n%s", out)

	entry := list[0]
	require.Equal(t, "/net/connman/iwd/phy0", jsonGetString(t, entry, "Path"))
	require.Equal(t, "phy0", jsonGetString(t, entry, "Name"))
	require.Equal(t, true, jsonGetBool(t, entry, "Powered"))
	require.Equal(t, "MockModel", jsonGetString(t, entry, "Model"))
	require.Equal(t, "MockVendor", jsonGetString(t, entry, "Vendor"))
}

// TestAdapterMock_ErrorMessageNotDuplicated guards end-to-end against the public
// error message restating a wrapped layer's frame. A bad SupportedModes payload
// produces a multi-layer error (variant conversion -> core -> public); the
// public frame and its details must each appear exactly once rather than being
// duplicated by the public-over-core wrapping.
func TestAdapterMock_ErrorMessageNotDuplicated(t *testing.T) {
	iwdmock.StartMockAdapterWithBadModes(t)

	out, err := runSpiderAdapter(t, "phy0", "supported-modes")
	require.Error(t, err, "output:\n%s", out)

	mustContainExactlyOnce(t, out, "adapter unavailable: Op=Adapter.SupportedModes:")
	mustContainExactlyOnce(t, out, "(failed querying iwd Adapter supported modes)")
}
