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

const adapterTimeout = 600 * time.Millisecond

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

func TestAdapterMock_GetPowered(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "powered")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, true, jsonGetBool(t, m, "Powered"))
}

func TestAdapterMock_SetPowered(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "powered", "off")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Powered"))
}

func TestAdapterMock_GetName(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "name")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "phy0", jsonGetString(t, m, "Value"))
}

func TestAdapterMock_GetModelProvided(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "model")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "MockModel", jsonGetString(t, m, "Value"))
}

func TestAdapterMock_GetModelNotImplemented(t *testing.T) {
	iwdmock.StartMockWithOmittedOptionals(t)

	_, out, err := runSpiderAdapterJSON(t, "phy0", "model")
	require.Error(t, err, "output:\n%s", out)
	mustContainAll(t, out, []string{"adapter unavailable", "dbus property error", "Op=Adapter.Model", "unknown property \"Model\""})
}

func TestAdapterMock_GetVendorProvided(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "vendor")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "MockVendor", jsonGetString(t, m, "Value"))
}

func TestAdapterMock_GetVendorNotImplemented(t *testing.T) {
	iwdmock.StartMockWithOmittedOptionals(t)

	_, out, err := runSpiderAdapterJSON(t, "phy0", "vendor")
	require.Error(t, err, "output:\n%s", out)
	mustContainAll(t, out, []string{"adapter unavailable", "dbus property error", "Op=Adapter.Vendor", "unknown property \"Vendor\""})
}

func TestAdapterMock_GetSupportedModes(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "supported-modes")
	require.NoError(t, err, "output:\n%s", out)
	require.ElementsMatch(t, []string{"station", "ap"}, jsonGetArray(t, m, "SupportedModes"))
}

func TestAdapterMock_SupportsMode_Supported(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "supports-mode", "station")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, true, jsonGetBool(t, m, "Value"))
}

func TestAdapterMock_SupportsMode_NotSupported(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "supports-mode", "ad-hoc")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Value"))
}

func TestAdapterMock_SupportsMode_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderAdapter(t, "phy0", "supports-mode", "42")
	require.Error(t, err, "output:\n%s", out)
	require.Contains(t, out, "invalid adapter mode \"42\"")
}

func TestAdapterMock_SupportsMode_Specific_Supported(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "supports-station")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, true, jsonGetBool(t, m, "Value"))
}

func TestAdapterMock_SupportsMode_Specific_NotSupported(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "supports-ad-hoc")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Value"))
}

func TestAdapterMock_ConcurrentModeChecks(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	const N = 100
	errs := make(chan error, N)

	for range N {
		go func() {
			_, _ = runSpiderAdapter(t, "phy0", "supported-modes")
			errs <- nil
		}()
	}

	for range N {
		require.NoError(t, <-errs)
	}
}

func TestAdapterMock_InvalidAdapter(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderAdapter(t, "phy9", "powered")
	require.Error(t, err, "output:\n%s", out)
	require.Contains(t, out, "adapter \"phy9\" not found")
}

func TestAdapterMock_BadSupportedModesType(t *testing.T) {
	iwdmock.StartMockAdapterWithBadModes(t)

	out, err := runSpiderAdapter(t, "phy0", "supported-modes")
	require.Error(t, err, "output:\n%s", out)
	mustContainAll(t, out, []string{"dbus variant conversion error", "variant=SupportedModes", "unexpected type"})
}

func TestAdapterMock_EventuallyPowered(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderAdapterJSON(t, "phy0", "powered", "true")
	require.NoError(t, err, "output:\n%s", out)
	require.True(t, jsonGetBool(t, m, "Powered"), "output:\n%s", out)

	require.Eventually(t, func() bool {
		m, out, err := runSpiderAdapterJSON(t, "phy0", "powered")
		require.NoError(t, err, "output:\n%s", out)
		return jsonGetBool(t, m, "Powered")
	}, adapterTimeout, pollInterval)
}

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

func TestAdapterMock_SubscribePoweredChanged_UnsubscribeStopsCallbacks(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	// SessionBus is where the iwd mock is registered.
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	adapter := newPublicMockAdapter(t, ctx, client, "phy0")
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
