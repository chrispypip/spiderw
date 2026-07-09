//go:build unit

package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseGlobalFlags covers each global-flag branch of the pure arg-scanning
// helper, including the alias spellings and the removal of consumed flags.
func TestParseGlobalFlags(t *testing.T) {
	t.Parallel()

	t.Run("JSON", func(t *testing.T) {
		for _, flag := range []string{"--json", "-json"} {
			app, rest := parseGlobalFlags([]string{flag, "adapter", "status"})
			require.True(t, app.Output.JSON)
			require.False(t, app.Session)
			require.False(t, app.Help)
			require.Equal(t, []string{"adapter", "status"}, rest)
		}
	})

	t.Run("Session", func(t *testing.T) {
		for _, flag := range []string{"--session", "-session"} {
			app, rest := parseGlobalFlags([]string{flag, "daemon"})
			require.True(t, app.Session)
			require.Equal(t, []string{"daemon"}, rest)
		}
	})

	t.Run("Help", func(t *testing.T) {
		for _, flag := range []string{"--help", "-help", "-h"} {
			app, _ := parseGlobalFlags([]string{"adapter", flag})
			require.True(t, app.Help)
		}
	})

	t.Run("NoGlobalFlagsPassThrough", func(t *testing.T) {
		app, rest := parseGlobalFlags([]string{"adapter", "phy0", "status"})
		require.False(t, app.Output.JSON)
		require.False(t, app.Session)
		require.False(t, app.Help)
		require.Equal(t, []string{"adapter", "phy0", "status"}, rest)
	})

	t.Run("Combined", func(t *testing.T) {
		app, rest := parseGlobalFlags([]string{"--json", "--session", "network", "status"})
		require.True(t, app.Output.JSON)
		require.True(t, app.Session)
		require.Equal(t, []string{"network", "status"}, rest)
	})
}

// Help short-circuits before any client is constructed, so these run with no
// D-Bus and no fake.
func TestHelp_ListsCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want []string
	}{
		{"daemon", []string{"Usage:", "spiderw daemon", "Commands:", "info", "version", "state-dir", "net-conf"}},
		{"adapter", []string{"Usage:", "spiderw adapter", "Commands:", "list", "status", "<adapter> status", "powered", "supported-modes", "supports-station", "monitor"}},
		{"device", []string{"Usage:", "spiderw device", "Commands:", "list", "status", "<device> status", "powered", "mode", "address", "adapter", "monitor"}},
		{"bss", []string{"Usage:", "spiderw bss", "Commands:", "list", "status", "<bss> status", "<bss> address"}},
		{"network", []string{"Usage:", "spiderw network", "Commands:", "list", "status", "<network> connect", "connected", "type", "bsses", "monitor"}},
		{"known-network", []string{"Usage:", "spiderw known-network", "Commands:", "list", "status", "autoconnect", "forget", "last-connected", "monitor"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			code := Run([]string{tc.name, "--help"}, &buf, &buf)
			require.Equal(t, 0, code, buf.String())

			out := buf.String()
			for _, w := range tc.want {
				require.Contains(t, out, w, "help output:\n%s", out)
			}
		})
	}
}
