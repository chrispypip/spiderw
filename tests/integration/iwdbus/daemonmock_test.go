//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// These tests exercise the daemon against a real D-Bus round trip. Per the
// integration testing convention, the public Go API is the baseline (it is the
// primary product surface and carries typed errors), while the CLI gets a
// thin layer covering only CLI-specific behavior (routing, output, exit codes).
// Exhaustive daemon parsing/normalization/error matrices live in the iwdbus and
// core unit tests and are not re-tested here.

// -----------------------------------------------------------------------------
// Public API against the mock
// -----------------------------------------------------------------------------

func newMockClient(t *testing.T, ctx context.Context) *spiderw.Client {
	t.Helper()

	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

func TestDaemonMock_Info(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	daemon := newMockClient(t, ctx).Daemon()
	require.NotNil(t, daemon)

	info, err := daemon.Info(ctx)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", info.Version)
	require.Equal(t, "/test/iwd/state", info.StateDirectory)
	require.True(t, info.NetworkConfigurationEnabled)

	// Convenience accessors agree with Info.
	version, err := daemon.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", version)

	stateDir, err := daemon.StateDirectory(ctx)
	require.NoError(t, err)
	require.Equal(t, "/test/iwd/state", stateDir)

	netConf, err := daemon.NetworkConfigurationEnabled(ctx)
	require.NoError(t, err)
	require.True(t, netConf)
}

func TestDaemonMock_Adapters(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	refs, err := newMockClient(t, ctx).Daemon().Adapters(ctx)
	require.NoError(t, err)
	require.Equal(t, []spiderw.AdapterRef{
		{Path: "/net/connman/iwd/phy0", Name: "phy0"},
	}, refs)
}

// TestDaemonMock_MalformedReply is the representative end-to-end malformed-reply
// case: a real D-Bus Info payload missing Version must surface as a typed
// invalid-state error through the public API. The full field/type matrix is
// unit-tested in internal/iwdbus and internal/core.
func TestDaemonMock_MalformedReply(t *testing.T) {
	iwdmock.StartMockWithMissingDaemonInfoFields(t, true, false, false)

	ctx := context.Background()
	_, err := newMockClient(t, ctx).Daemon().Version(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrInvalidState)

	var pe *spiderw.Error
	require.ErrorAs(t, err, &pe)
	require.Equal(t, spiderw.ResourceDaemon, pe.Resource)
}

func TestDaemonMock_NoDaemon(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockWithoutDaemon(t, tmpDir)

	// With no daemon object exported, client construction itself fails.
	_, err := spiderw.NewClient(context.Background(), spiderw.SessionBus)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrInternal)
}

// TestDaemonMock_ConcurrentInfo exercises concurrency safety of a single client
// against the real bus. (This replaces a former variant that spawned 100 CLI
// subprocesses; one shared client is both cheaper and a more direct test.)
func TestDaemonMock_ConcurrentInfo(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	daemon := newMockClient(t, ctx).Daemon()

	const N = 100
	errCh := make(chan error, N)
	for range N {
		go func() {
			_, err := daemon.Info(ctx)
			errCh <- err
		}()
	}
	for range N {
		require.NoError(t, <-errCh)
	}
}

// -----------------------------------------------------------------------------
// CLI (`spiderw daemon …`) against the mock — thin, CLI-specific coverage only
// -----------------------------------------------------------------------------

func TestDaemonMock_CLI_Info(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDaemonJSON(t, "info")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "1.0.0", jsonGetString(t, m, "Version"))
	require.Equal(t, "/test/iwd/state", jsonGetString(t, m, "StateDirectory"))
	require.Equal(t, true, jsonGetBool(t, m, "NetworkConfigurationEnabled"))
}

func runSpiderDaemon(t *testing.T, args ...string) (string, error) {
	t.Helper()

	return runSpider(t, append([]string{"daemon"}, args...)...)
}

func runSpiderDaemonJSON(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()

	return runSpiderJSON(t, append([]string{"daemon"}, args...)...)
}
