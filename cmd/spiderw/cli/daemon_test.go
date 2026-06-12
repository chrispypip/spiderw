//go:build unit

package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

func fakeWithDaemon() *fakeClient {
	return &fakeClient{
		daemon: &fakeDaemon{
			info: &spiderw.DaemonInfo{
				Version:                     "1.0.0",
				StateDirectory:              "/var/lib/iwd",
				NetworkConfigurationEnabled: true,
			},
		},
	}
}

func TestDaemonCmd_Info_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDaemon(), nil, false, "daemon", "info")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "1.0.0")
	require.Contains(t, out, "/var/lib/iwd")
	require.Contains(t, out, "true")
}

func TestDaemonCmd_Info_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDaemon(), nil, true, "daemon", "info")
	require.Equal(t, 0, code, out)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Equal(t, "1.0.0", got["Version"])
	require.Equal(t, "/var/lib/iwd", got["StateDirectory"])
	require.Equal(t, true, got["NetworkConfigurationEnabled"])
}

func TestDaemonCmd_Scalars(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDaemon(), nil, false, "daemon", "version")
	require.Equal(t, 0, code, out)
	require.Equal(t, "1.0.0\n", out)

	out, code = driveCLI(fakeWithDaemon(), nil, false, "daemon", "state-dir")
	require.Equal(t, 0, code, out)
	require.Equal(t, "/var/lib/iwd\n", out)

	out, code = driveCLI(fakeWithDaemon(), nil, false, "daemon", "net-conf")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true\n", out)
}

func TestDaemonCmd_BackendError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{daemon: &fakeDaemon{err: errors.New("daemon boom")}}
	out, code := driveCLI(fc, nil, false, "daemon", "info")
	require.Equal(t, 1, code)
	require.Contains(t, out, "daemon boom")
}

func TestDaemonCmd_UsageError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDaemon(), nil, false, "daemon", "info", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown daemon info argument")
}

func TestDaemonCmd_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDaemon(), nil, false, "daemon", "bogus")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown subcommand")
}

func TestDaemonCmd_ClientConstructionError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(nil, errors.New("no session bus"), false, "daemon", "info")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no session bus")
}
