//go:build unit

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

// TestMonitorPathHelpers covers the adapters between Properties() (which returns
// resolved refs) and the monitor printers (which take raw paths, matching the
// subscribe callbacks). Their nil handling decides whether a user sees a path or
// "none", so both sides matter.
func TestMonitorPathHelpers(t *testing.T) {
	t.Parallel()

	t.Run("refPath", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, refPath(nil), "a disconnected station has no network ref")
		got := refPath(&spiderw.NetworkRef{Path: "/n", Name: "MyNet"})
		require.NotNil(t, got)
		require.Equal(t, "/n", *got, "the raw path is streamed, not the resolved name")
	})

	t.Run("bssRefPath", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, bssRefPath(nil))
		got := bssRefPath(&spiderw.BasicServiceSetRef{Path: "/b", Address: "aa:bb:cc:dd:ee:ff"})
		require.NotNil(t, got)
		require.Equal(t, "/b", *got)
	})

	t.Run("bssRefPaths", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, bssRefPaths(nil))
		require.Nil(t, bssRefPaths([]spiderw.BasicServiceSetRef{}))
		require.Equal(t, []string{"/a", "/b"}, bssRefPaths([]spiderw.BasicServiceSetRef{
			{Path: "/a", Address: "aa:bb:cc:dd:ee:ff"},
			{Path: "/b", Address: "bb:cc:dd:ee:ff:00"},
		}))
	})

	t.Run("optionalPathText", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "absent", optionalPathText(nil, "absent"))
		empty := ""
		require.Equal(t, "absent", optionalPathText(&empty, "absent"), "an empty path is absent too")
		p := "/x"
		require.Equal(t, "/x", optionalPathText(&p, "absent"))
	})

	t.Run("pathListText", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "none", pathListText(nil))
		require.Equal(t, "none", pathListText([]string{}))
		require.Equal(t, "/a, /b", pathListText([]string{"/a", "/b"}))
	})
}
