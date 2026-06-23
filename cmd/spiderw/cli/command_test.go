//go:build unit

package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

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
