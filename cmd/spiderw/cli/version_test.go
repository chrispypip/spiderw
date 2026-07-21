//go:build unit

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCmd_PrintsResolvedVersion(t *testing.T) {
	// Not parallel: it sets the package-level `version`.
	orig := version
	t.Cleanup(func() { version = orig })

	version = "v9.9.9"
	out, code := driveCLI(nil, nil, false, "version")
	require.Equal(t, 0, code, out)
	require.Equal(t, "v9.9.9\n", out)
}

func TestVersionCmd_RejectsArgs(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(nil, nil, false, "version", "extra")
	require.Equal(t, 1, code, out)
	require.Contains(t, out, "usage: spiderw version")
}

func TestResolveVersion_FallsBackToBuildInfo(t *testing.T) {
	// With no ldflags stamp, resolveVersion consults the build info. Under `go
	// test` that yields a non-empty value ("(devel)"), never the "unknown" floor.
	orig := version
	t.Cleanup(func() { version = orig })

	version = ""
	require.NotEmpty(t, resolveVersion())
	require.NotEqual(t, "unknown", resolveVersion())
}
