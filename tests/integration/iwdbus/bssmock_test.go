//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// mockBSSes is the set of basic service sets the mock exports (nested under their
// networks), in the daemon's path-sorted enumeration order.
var mockBSSes = []spiderw.BasicServiceSetRef{
	{Path: "/net/connman/iwd/0/3/4b6e6f776e4e6574_psk/deadbeefcafe", Address: "de:ad:be:ef:ca:fe"},
	{Path: "/net/connman/iwd/0/3/4f70656e4e6574_open/112233445566", Address: "11:22:33:44:55:66"},
	{Path: "/net/connman/iwd/0/3/4f70656e4e6574_open/778899aabbcc", Address: "77:88:99:aa:bb:cc"},
}

// newPublicMockBSS resolves a public BasicServiceSet handle for the mock BSS
// with the given address.
func newPublicMockBSS(t *testing.T, ctx context.Context, client *spiderw.Client, address string) *spiderw.BasicServiceSet {
	t.Helper()

	daemon := client.Daemon()
	require.NotNil(t, daemon)

	refs, err := daemon.BasicServiceSets(ctx)
	require.NoError(t, err)

	for _, ref := range refs {
		if ref.Address != address {
			continue
		}

		bss, err := client.BasicServiceSet(ctx, ref.Path)
		require.NoError(t, err)
		require.NotNil(t, bss)
		return bss
	}

	t.Fatalf("mock basic service set %q not found in refs: %#v", address, refs)
	return nil
}

// -----------------------------------------------------------------------------
// Public client against the mock
// -----------------------------------------------------------------------------

func TestBSSMock_DaemonBasicServiceSets(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	refs, err := client.Daemon().BasicServiceSets(ctx)
	require.NoError(t, err)
	// Enumeration returns every BSS, in path-sorted order.
	require.Equal(t, mockBSSes, refs)
}

func TestBSSMock_Properties(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	// Each BSS resolves to a live handle reporting its own address.
	for _, want := range mockBSSes {
		b := newPublicMockBSS(t, ctx, client, want.Address)

		addr, err := b.Address(ctx)
		require.NoError(t, err)
		require.Equal(t, want.Address, addr)

		props, err := b.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, want.Address, props.Address)
	}
}

func TestBSSMock_AllBasicServiceSets(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	bsses, err := client.AllBasicServiceSets(ctx)
	require.NoError(t, err)
	require.Len(t, bsses, len(mockBSSes))

	// Order is preserved and each handle is live.
	for i, want := range mockBSSes {
		require.Equal(t, want.Path, bsses[i].Path())

		addr, err := bsses[i].Address(ctx)
		require.NoError(t, err)
		require.Equal(t, want.Address, addr)
	}
}

func TestBSSMock_AllBasicServiceSets_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockWithoutBSS(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	refs, err := client.Daemon().BasicServiceSets(ctx)
	require.NoError(t, err)
	require.Empty(t, refs)

	bsses, err := client.AllBasicServiceSets(ctx)
	require.NoError(t, err)
	require.Empty(t, bsses)
}

// -----------------------------------------------------------------------------
// CLI (`spiderw bss …`) against the mock
// -----------------------------------------------------------------------------

func findBSSStatusEntry(t *testing.T, list []map[string]any, path string) map[string]any {
	t.Helper()

	for _, entry := range list {
		if p, ok := entry["Path"].(string); ok && p == path {
			return entry
		}
	}

	t.Fatalf("basic service set %q not found in status output: %#v", path, list)
	return nil
}

// TestBSSMock_StatusJSON is the representative end-to-end CLI smoke for the BSS:
// it drives `bss status --json` through the full real-D-Bus stack
// (Client.AllBasicServiceSets + per-BSS Properties) and asserts the structured
// output for every exported BSS. Per-command behavior, output formatting, ref
// resolution, and error mapping are covered by the fast in-process unit tests in
// cmd/spiderw/cli.
func TestBSSMock_StatusJSON(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	list, out, err := runSpiderJSONArray(t, "bss", "status")
	require.NoError(t, err, "output:\n%s", out)
	require.Len(t, list, len(mockBSSes), "output:\n%s", out)

	for _, want := range mockBSSes {
		entry := findBSSStatusEntry(t, list, want.Path)
		require.Equal(t, want.Address, jsonGetString(t, entry, "Address"))
	}
}

// TestBSSMock_CLI_SingleAddress drives the single-BSS lookup path end-to-end
// (`spiderw bss <ref> address`), which resolves a ref and constructs one handle
// via Client.BasicServiceSet — the realClient shim not exercised by the
// enumeration-based `bss status` smoke.
func TestBSSMock_CLI_SingleAddress(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ref := mockBSSes[0]
	out, err := runSpider(t, "bss", ref.Address, "address")
	require.NoError(t, err, "output:\n%s", out)
	require.Contains(t, out, ref.Address)
}
