//go:build unit

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

// fakeWithNetwork builds a client exposing the three networks the mock models:
// an open network, a known (provisioned) network, and an unknown secured network
// that needs a credentials agent to connect.
func fakeWithNetwork() *fakeClient {
	open := &fakeNetwork{
		path: "/net/connman/iwd/phy0/wlan0/open",
		props: &spiderw.NetworkProperties{
			Name:   "OpenNet",
			Device: spiderw.DeviceRef{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"},
			Type:   spiderw.NetworkTypeOpen,
			ExtendedServiceSet: []spiderw.BasicServiceSetRef{
				{Path: "/net/connman/iwd/phy0/wlan0/aabbccddeeff", Address: "aa:bb:cc:dd:ee:ff"},
				{Path: "/net/connman/iwd/phy0/wlan0/bbccddeeff00", Address: "bb:cc:dd:ee:ff:00"},
			},
		},
	}
	knownNet := &fakeNetwork{
		path: "/net/connman/iwd/phy0/wlan0/known_psk",
		props: &spiderw.NetworkProperties{
			Name:         "KnownNet",
			Device:       spiderw.DeviceRef{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"},
			Type:         spiderw.NetworkTypePSK,
			KnownNetwork: new("/net/connman/iwd/known_networks/1"),
		},
	}
	secured := &fakeNetwork{
		path: "/net/connman/iwd/phy0/wlan0/secured_psk",
		props: &spiderw.NetworkProperties{
			Name:   "SecuredNet",
			Device: spiderw.DeviceRef{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"},
			Type:   spiderw.NetworkTypePSK,
		},
	}

	return &fakeClient{
		daemon: &fakeDaemon{networks: []spiderw.NetworkRef{
			{Path: knownNet.path, Name: "KnownNet"},
			{Path: open.path, Name: "OpenNet"},
			{Path: secured.path, Name: "SecuredNet"},
		}},
		networks: map[string]networkAPI{
			open.path:     open,
			knownNet.path: knownNet,
			secured.path:  secured,
		},
		allNetworks: []networkAPI{knownNet, open, secured},
	}
}

func TestNetworkCmd_Status_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, true, "network", "status")
	require.Equal(t, 0, code, out)

	var entries []networkStatusEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.Len(t, entries, 3)
}

func TestNetworkCmd_Status_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "OpenNet")
}

func TestNetworkCmd_List(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "list")
	require.Equal(t, 0, code, out)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
	require.Contains(t, out, "OpenNet")
	require.Contains(t, out, "SecuredNet")
}

func TestNetworkCmd_Connect_Open(t *testing.T) {
	t.Parallel()

	fc := fakeWithNetwork()
	out, code := driveCLI(fc, nil, false, "network", "OpenNet", "connect")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))
	// An open network needs no agent.
	require.Nil(t, fc.registeredCfg)
}

func TestNetworkCmd_Connect_Secured_PassphraseFlag(t *testing.T) {
	t.Parallel()

	fc := fakeWithNetwork()
	out, code := driveConnect(fc, "", nil, "network", "SecuredNet", "connect", "--passphrase=hunter2")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))

	// The CLI registered an agent whose passphrase callback returns the flag
	// value, then unregistered it.
	require.NotNil(t, fc.registeredCfg)
	require.NotNil(t, fc.registeredCfg.Passphrase)
	secret, err := fc.registeredCfg.Passphrase(context.Background(), "/net/connman/iwd/phy0/wlan0/secured_psk")
	require.NoError(t, err)
	require.Equal(t, "hunter2", secret)
	require.NotNil(t, fc.agent)
	require.True(t, fc.agent.unregistered)
}

func TestNetworkCmd_Connect_Secured_PassphraseStdin(t *testing.T) {
	t.Parallel()

	fc := fakeWithNetwork()
	out, code := driveConnect(fc, "topsecret\n", nil, "network", "SecuredNet", "connect", "--passphrase-stdin")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))

	require.NotNil(t, fc.registeredCfg)
	secret, err := fc.registeredCfg.Passphrase(context.Background(), "p")
	require.NoError(t, err)
	require.Equal(t, "topsecret", secret)
}

func TestNetworkCmd_Connect_Secured_Prompt(t *testing.T) {
	t.Parallel()

	fc := fakeWithNetwork()
	prompt := func(string) (string, error) { return "prompted-secret", nil }
	out, code := driveConnect(fc, "", prompt, "network", "SecuredNet", "connect")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))

	require.NotNil(t, fc.registeredCfg)
	secret, err := fc.registeredCfg.Passphrase(context.Background(), "p")
	require.NoError(t, err)
	require.Equal(t, "prompted-secret", secret)
}

func TestNetworkCmd_Connect_Secured_BothSourcesRejected(t *testing.T) {
	t.Parallel()

	fc := fakeWithNetwork()
	out, code := driveConnect(fc, "x\n", nil, "network", "SecuredNet", "connect", "--passphrase=y", "--passphrase-stdin")
	require.Equal(t, 1, code)
	require.Contains(t, out, "only one of --passphrase")
	require.Nil(t, fc.registeredCfg)
}

func TestNetworkCmd_Connect_Secured_RegisterFails(t *testing.T) {
	t.Parallel()

	fc := fakeWithNetwork()
	fc.registerErr = errors.New("agent slot taken")
	out, code := driveConnect(fc, "", func(string) (string, error) { return "s", nil }, "network", "SecuredNet", "connect")
	require.Equal(t, 1, code)
	require.Contains(t, out, "agent slot taken")
}

func TestNetworkCmd_Connect_Known_NoAgent(t *testing.T) {
	t.Parallel()

	// A known (provisioned) secured network connects without an agent.
	fc := fakeWithNetwork()
	out, code := driveCLI(fc, nil, false, "network", "KnownNet", "connect")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))
	require.Nil(t, fc.registeredCfg)
}

func TestNetworkCmd_Type(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "type")
	require.Equal(t, 0, code, out)
	require.Equal(t, "open", strings.TrimSpace(out))
}

func TestNetworkCmd_BSSes(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "bsses")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "/net/connman/iwd/phy0/wlan0/aabbccddeeff")
	require.Contains(t, out, "/net/connman/iwd/phy0/wlan0/bbccddeeff00")
}

func TestNetworkCmd_Connected(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "connected")
	require.Equal(t, 0, code, out)
	require.Equal(t, "false", strings.TrimSpace(out))
}

func TestNetworkCmd_Device(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "device")
	require.Equal(t, 0, code, out)
	require.Equal(t, "/net/connman/iwd/phy0/wlan0", strings.TrimSpace(out))
}

func TestNetworkCmd_Name(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "name")
	require.Equal(t, 0, code, out)
	require.Equal(t, "OpenNet", strings.TrimSpace(out))
}

func TestNetworkCmd_KnownNetwork(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "KnownNet", "known-network")
	require.Equal(t, 0, code, out)
	require.Equal(t, "/net/connman/iwd/known_networks/1", strings.TrimSpace(out))
}

func TestNetworkCmd_SingleStatus_ByName(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, true, "network", "OpenNet", "status")
	require.Equal(t, 0, code, out)

	var entries []networkStatusEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "OpenNet", entries[0].Name)
	require.Equal(t, "open", entries[0].Type)
}

func TestNetworkCmd_EnumerationError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{allNetErr: errors.New("enumeration boom")}
	out, code := driveCLI(fc, nil, false, "network", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}

func TestNetworkCmd_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "powered")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown network command")
}

// TestPrintNetworkConnectedLine covers the monitor output helper directly (the
// monitor command blocks on an OS signal and is not drivable in-process).
func TestPrintNetworkConnectedLine(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex

	app, buf := appWithBuffer(false)
	require.NoError(t, printNetworkConnectedLine(app, "OpenNet", true, &mu))
	require.Equal(t, "connected=true\n", buf.String())

	appJSON, bufJSON := appWithBuffer(true)
	require.NoError(t, printNetworkConnectedLine(appJSON, "OpenNet", false, &mu))
	require.Contains(t, bufJSON.String(), `"Connected":false`)
}

// TestPrintNetworkMonitorLines covers the new network monitor output helpers
// directly (the monitor command blocks on an OS signal).
func TestPrintNetworkMonitorLines(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	kn := "/net/connman/iwd/known_network/abc"

	t.Run("known-network", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		// Human output shows the resolved name, not the raw path.
		require.NoError(t, printNetworkKnownNetworkLine(app, "MyNet", &nameRef{Name: "KnownNet", Path: kn}, &mu))
		require.Equal(t, "known-network=KnownNet\n", buf.String())

		// An unresolvable ref falls back to the path rather than printing blank.
		appPath, bufPath := appWithBuffer(false)
		require.NoError(t, printNetworkKnownNetworkLine(appPath, "MyNet", &nameRef{Path: kn}, &mu))
		require.Equal(t, "known-network="+kn+"\n", bufPath.String())

		// A forgotten network reports nil, which must read as words rather than an
		// empty value.
		appOff, bufOff := appWithBuffer(false)
		require.NoError(t, printNetworkKnownNetworkLine(appOff, "MyNet", nil, &mu))
		require.Equal(t, "known-network=none (not saved)\n", bufOff.String())

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printNetworkKnownNetworkLine(appJSON, "MyNet", nil, &mu))
		require.Contains(t, bufJSON.String(), `"KnownNetwork":null`)
	})

	t.Run("bsses", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		require.NoError(t, printNetworkBSSesLine(app, "MyNet", []string{"/bss/a", "/bss/b"}, &mu))
		require.Equal(t, "bsses=/bss/a, /bss/b\n", buf.String())

		appOff, bufOff := appWithBuffer(false)
		require.NoError(t, printNetworkBSSesLine(appOff, "MyNet", nil, &mu))
		require.Equal(t, "bsses=none\n", bufOff.String())

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printNetworkBSSesLine(appJSON, "MyNet", []string{"/bss/a"}, &mu))
		require.Contains(t, bufJSON.String(), `"ExtendedServiceSet":["/bss/a"]`)
	})
}

func TestNetworkCmd_Monitor_BadArgs(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"network", "/net/connman/iwd/phy0/wlan0/open", "monitor"},
		{"network", "/net/connman/iwd/phy0/wlan0/open", "monitor", "bogus"},
		{"network", "/net/connman/iwd/phy0/wlan0/open", "monitor", "connected", "extra"},
	} {
		out, code := driveCLI(fakeWithNetwork(), nil, false, args...)
		require.Equal(t, 1, code, out)
		require.Contains(t, out, "usage:")
	}
}

// TestStreamNetworkProperty drives the monitor's non-blocking core: the current
// value prints, and each target wires its own subscription.
func TestStreamNetworkProperty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	knPath := "/net/connman/iwd/known_network/evented"
	bssPath := "/net/connman/iwd/phy0/wlan0/ffeeddccbbaa"

	// The open network has no KnownNetwork, so "known-network" seeds as "not saved"
	// and then streams the saved path — the save transition, end to end.
	newFake := func() *fakeNetwork {
		n := fakeWithNetwork().networks["/net/connman/iwd/phy0/wlan0/open"].(*fakeNetwork)
		n.knownNetEvent = &cliOptStringEvent{v: &knPath}
		n.essEvent = &cliStringSliceEvent{v: []string{bssPath}}
		return n
	}

	for _, tc := range []struct {
		what        string
		wantSeed    string
		wantEvent   string
		wantSubcall string
	}{
		{"connected", "connected=", "connected=", "SubscribeConnectedChanged"},
		{"known-network", "known-network=none (not saved)", "known-network=" + knPath, "SubscribeKnownNetworkChanged"},
		{"bsses", "bsses=", "bsses=" + bssPath, "SubscribeExtendedServiceSetChanged"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			t.Parallel()
			n := newFake()
			app, buf := appWithBuffer(false)
			var mu sync.Mutex

			unsubscribe, err := streamNetworkProperty(ctx, app, "OpenNet", tc.what, n, monitorResolver{}, &mu)
			require.NoError(t, err)
			require.NoError(t, unsubscribe.Unsubscribe())

			out := buf.String()
			require.Contains(t, out, tc.wantSeed, "the current value must print first")
			require.Contains(t, out, tc.wantEvent, "a subsequent change must print")
			require.Equal(t, tc.wantSubcall, n.subscribed,
				"target %q must subscribe to its own property", tc.what)
		})
	}
}

func TestStreamNetworkProperty_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var mu sync.Mutex
	app, _ := appWithBuffer(false)

	bad := &fakeNetwork{path: "/n", err: errors.New("read failed")}
	_, err := streamNetworkProperty(ctx, app, "OpenNet", "known-network", bad, monitorResolver{}, &mu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read failed")

	sub := fakeWithNetwork().networks["/net/connman/iwd/phy0/wlan0/open"].(*fakeNetwork)
	sub.subscribeErr = errors.New("subscribe failed")
	_, err = streamNetworkProperty(ctx, app, "OpenNet", "bsses", sub, monitorResolver{}, &mu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscribe failed")

	ok := fakeWithNetwork().networks["/net/connman/iwd/phy0/wlan0/open"].(*fakeNetwork)
	_, err = streamNetworkProperty(ctx, app, "OpenNet", "bogus", ok, monitorResolver{}, &mu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage:")
}

func TestParseNetworkMonitorTarget(t *testing.T) {
	t.Parallel()

	for _, what := range networkMonitorTargets {
		got, err := parseNetworkMonitorTarget([]string{what})
		require.NoError(t, err)
		require.Equal(t, what, got)
	}
	for _, args := range [][]string{nil, {"bogus"}, {"connected", "extra"}} {
		_, err := parseNetworkMonitorTarget(args)
		require.Error(t, err)
	}
}
