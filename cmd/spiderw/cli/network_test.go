//go:build unit

package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

// fakeWithNetwork builds a client exposing the three networks the mock models:
// an open network, a known (provisioned) network, and an unknown secured network
// whose Connect fails.
func fakeWithNetwork() *fakeClient {
	known := "/net/connman/iwd/known_networks/1"

	open := &fakeNetwork{
		path: "/net/connman/iwd/phy0/wlan0/open",
		props: &spiderw.NetworkProperties{
			Name:               "OpenNet",
			Device:             "/net/connman/iwd/phy0/wlan0",
			Type:               spiderw.SecurityTypeOpen,
			ExtendedServiceSet: []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff", "/net/connman/iwd/phy0/wlan0/bbccddeeff00"},
		},
	}
	knownNet := &fakeNetwork{
		path: "/net/connman/iwd/phy0/wlan0/known_psk",
		props: &spiderw.NetworkProperties{
			Name:         "KnownNet",
			Device:       "/net/connman/iwd/phy0/wlan0",
			Type:         spiderw.SecurityTypePSK,
			KnownNetwork: &known,
		},
	}
	secured := &fakeNetwork{
		path: "/net/connman/iwd/phy0/wlan0/secured_psk",
		props: &spiderw.NetworkProperties{
			Name:   "SecuredNet",
			Device: "/net/connman/iwd/phy0/wlan0",
			Type:   spiderw.SecurityTypePSK,
		},
		connectErr: errors.New("no credentials agent registered"),
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

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "OpenNet", "connect")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))
}

func TestNetworkCmd_Connect_SecuredFails(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithNetwork(), nil, false, "network", "SecuredNet", "connect")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no credentials agent registered")
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
