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

// fakeWithBSS builds a client exposing multiple basic service sets, mirroring iwd
// reporting one BSS per access point a device can hear.
func fakeWithBSS() *fakeClient {
	bss0 := &fakeBSS{
		path:  "/net/connman/iwd/phy0/wlan0/aabbccddeeff",
		props: &spiderw.BasicServiceSetProperties{Address: "11:22:33:44:55:66"},
	}
	bss1 := &fakeBSS{
		path:  "/net/connman/iwd/phy0/wlan0/bbccddeeff00",
		props: &spiderw.BasicServiceSetProperties{Address: "77:88:99:aa:bb:cc"},
	}
	return &fakeClient{
		daemon: &fakeDaemon{bsses: []spiderw.BasicServiceSetRef{
			{Path: bss0.path, Address: bss0.props.Address},
			{Path: bss1.path, Address: bss1.props.Address},
		}},
		bsses:    map[string]bssAPI{bss0.path: bss0, bss1.path: bss1},
		allBSSes: []bssAPI{bss0, bss1},
	}
}

func TestBSSCmd_Status_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithBSS(), nil, false, "bss", "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "11:22:33:44:55:66")
	require.Contains(t, out, "/net/connman/iwd/phy0/wlan0/aabbccddeeff")
	require.Contains(t, out, "77:88:99:aa:bb:cc")
	require.Contains(t, out, "/net/connman/iwd/phy0/wlan0/bbccddeeff00")
}

func TestBSSCmd_Status_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithBSS(), nil, true, "bss", "status")
	require.Equal(t, 0, code, out)

	var entries []bssStatusEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.Len(t, entries, 2)
	require.Equal(t, "11:22:33:44:55:66", entries[0].Address)
	require.Equal(t, "77:88:99:aa:bb:cc", entries[1].Address)
}

func TestBSSCmd_SingleStatus_ByAddress(t *testing.T) {
	t.Parallel()

	// A specific address resolves to exactly one BSS out of the many available.
	out, code := driveCLI(fakeWithBSS(), nil, true, "bss", "77:88:99:aa:bb:cc", "status")
	require.Equal(t, 0, code, out)

	var entries []bssStatusEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, "77:88:99:aa:bb:cc", entries[0].Address)
	require.Equal(t, "/net/connman/iwd/phy0/wlan0/bbccddeeff00", entries[0].Path)
}

func TestBSSCmd_List(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithBSS(), nil, false, "bss", "list")
	require.Equal(t, 0, code, out)
	// Every BSS is listed.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, out, "11:22:33:44:55:66")
	require.Contains(t, out, "77:88:99:aa:bb:cc")
}

func TestBSSCmd_Address(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithBSS(), nil, false, "bss", "11:22:33:44:55:66", "address")
	require.Equal(t, 0, code, out)
	require.Equal(t, "11:22:33:44:55:66", strings.TrimSpace(out))
}

func TestBSSCmd_EnumerationError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{allBSSErr: errors.New("enumeration boom")}
	out, code := driveCLI(fc, nil, false, "bss", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}

func TestBSSCmd_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithBSS(), nil, false, "bss", "11:22:33:44:55:66", "powered")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown bss command")
}
