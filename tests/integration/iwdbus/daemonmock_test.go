//go:build integration

package integration_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

func TestDaemonMock_GetInfo(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDaemonJSON(t, "info")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "1.0.0", jsonGetString(t, m, "Version"))
	require.Equal(t, "/test/iwd/state", jsonGetString(t, m, "StateDirectory"))
	require.Equal(t, true, jsonGetBool(t, m, "NetworkConfigurationEnabled"))
}

func TestDaemonMock_GetVersion(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDaemonJSON(t, "version")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "1.0.0", jsonGetString(t, m, "Version"))
}

func TestDaemonMock_GetStateDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDaemonJSON(t, "state-dir")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "/test/iwd/state", jsonGetString(t, m, "StateDirectory"))
}

func TestDaemonMock_IsNetworkConfigurationEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDaemonJSON(t, "net-conf")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, true, jsonGetBool(t, m, "NetworkConfigurationEnabled"))
}

func TestDaemonMock_NoDaemon(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockWithoutDaemon(t, tmpDir)

	out, err := runSpiderDaemon(t, "info")
	require.Error(t, err)
	mustContainAll(t, out, []string{"internal error", "operation failed", "Op=NewClient", "iwd daemon interface not available"})
}

func TestDaemonMock_MissingVersion(t *testing.T) {
	iwdmock.StartMockWithMissingDaemonInfoFields(t, true, false, false)

	out, err := runSpiderDaemon(t, "version")
	require.Error(t, err)
	mustContainAll(t, out, []string{"invalid state", "missing or invalid Version field", "daemon returned empty Version"})
}

func TestDaemonMock_MissingStateDir(t *testing.T) {
	iwdmock.StartMockWithMissingDaemonInfoFields(t, false, true, false)

	out, err := runSpiderDaemon(t, "state-dir")
	require.Error(t, err)
	mustContainAll(t, out, []string{"invalid state", "missing or invalid StateDirectory field", "daemon returned empty StateDirectory"})
}

func TestDaemonMock_MissingNetConf(t *testing.T) {
	iwdmock.StartMockWithMissingDaemonInfoFields(t, false, false, true)

	out, err := runSpiderDaemon(t, "net-conf")
	require.NoError(t, err)
	mustContain(t, out, "false")
}

func TestDaemonMock_GetInfo_MissingInfo(t *testing.T) {
	iwdmock.StartMockWithMissingDaemonInfoFields(t, true, false, true)

	out, err := runSpiderDaemon(t, "info")
	require.Error(t, err)
	mustContainAll(t, out, []string{"invalid state", "missing or invalid", "daemon returned empty"})
}

func TestDaemonMock_GetInfo_ExtraFields(t *testing.T) {
	iwdmock.StartMockWithExtraDaemonInfoFields(t)

	m, out, err := runSpiderDaemonJSON(t, "info")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "1.0.0", jsonGetString(t, m, "Version"))
	require.Equal(t, "/test/iwd/state", jsonGetString(t, m, "StateDirectory"))
	require.Equal(t, true, jsonGetBool(t, m, "NetworkConfigurationEnabled"))
}

func TestDaemonMock_GetVersion_WrongType(t *testing.T) {
	iwdmock.StartMockWithBadDaemonInfoFields(t, true, false, false)

	out, err := runSpiderDaemon(t, "version")
	require.Error(t, err)
	mustContainAll(t, out, []string{"Version", "expected string"})
}

func TestDaemonMock_GetStateDirectory_WrongType(t *testing.T) {
	iwdmock.StartMockWithBadDaemonInfoFields(t, false, true, false)

	out, err := runSpiderDaemon(t, "state-dir")
	require.Error(t, err)
	mustContainAll(t, out, []string{"StateDirectory", "expected string"})
}

func TestDaemonMock_GetNetConf_WrongType(t *testing.T) {
	iwdmock.StartMockWithBadDaemonInfoFields(t, false, false, true)

	out, err := runSpiderDaemon(t, "net-conf")
	require.Error(t, err)
	mustContainAll(t, out, []string{"NetworkConfigurationEnabled", "expected bool"})
}

func TestDaemonMock_ConcurrentGetInfo(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	const N = 100
	errCh := make(chan error, N)

	for range N {
		go func() {
			out, err := runSpiderDaemon(t, "info")
			if err != nil {
				errCh <- fmt.Errorf("call failed: %w: %s", err, out)
			} else {
				errCh <- nil
			}
		}()
	}

	for range N {
		err := <-errCh
		require.NoError(t, err)
	}
}

func TestDaemonMock_GetInfo_BadPayloadType(t *testing.T) {
	iwdmock.StartMockWithDaemonGetInfoReturningBadType(t)

	out, err := runSpiderDaemon(t, "info")
	require.Error(t, err)
	mustContainAll(t, out, []string{"failed", "parse", "Version", "expected string"})
}

func TestDaemonMock_GetInfo_DBusFailure(t *testing.T) {
	iwdmock.StartMockWithDaemonFailingCalls(t)

	out, err := runSpiderDaemon(t, "info")
	require.Error(t, err)
	mustContainAll(t, out, []string{"GetInfo", "fail"})
}

func TestDaemonMock_InvalidSubcommand(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderDaemon(t, "info", "bogus")

	require.Error(t, err)
	mustContain(t, out, "unknown")
}

func runSpiderDaemon(t *testing.T, args ...string) (string, error) {
	t.Helper()

	return runSpider(t, append([]string{"daemon"}, args...)...)
}

func runSpiderDaemonJSON(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()

	return runSpiderJSON(t, append([]string{"daemon"}, args...)...)
}
