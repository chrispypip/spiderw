//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

func TestSignalFirehose_DaemonSurvives(t *testing.T) {
	iwdmock.StartMockFirehose(t)

	for range 40 {
		out, err := runSpiderDaemon(t, "info")
		require.NoError(t, err, "daemon info failed under firehose load: %s", out)
		require.Contains(t, out, "Version:")
		require.Contains(t, out, "StateDirectory:")
		require.Contains(t, out, "NetworkConfigurationEnabled:")
	}
}

func TestSignalFirehose_AdapterSurvives(t *testing.T) {
	iwdmock.StartMockFirehose(t)

	for range 20 {
		out, err := runSpiderAdapter(t, "list")
		require.NoError(t, err, "adapter list failed under firehose load: %s", out)
		require.Contains(t, out, "phy0")
		require.Contains(t, out, "/net/connman/iwd/phy0")

		out, err = runSpiderAdapter(t, "phy0", "powered")
		require.NoError(t, err, "adapter powered failed under firehose load: %s", out)
		require.Contains(t, []string{"true\n", "false\n"}, out)

		out, err = runSpiderAdapter(t, "phy0", "supported-modes")
		require.NoError(t, err, "adapter supported-modes failed under firehose load: %s", out)
		require.Contains(t, out, "station")
		require.Contains(t, out, "ap")

		out, err = runSpiderAdapter(t, "phy0", "supports-mode", "station")
		require.NoError(t, err, "adapter supports-mode station failed under firehose load: %s", out)
		require.Equal(t, "true\n", out)
	}
}

func TestSignalFirehose_PublicAdapterSubscribePoweredReceivesSignals(t *testing.T) {
	iwdmock.StartMockFirehose(t)

	ctx := context.Background()
	// SessionBus is where the iwd mock is registered.
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	adapter := newPublicMockAdapter(t, ctx, client, "phy0")
	fired := make(chan struct{}, 10)

	unsubscribe, err := adapter.SubscribePoweredChanged(ctx, func(bool) {
		select {
		case fired <- struct{}{}:
		default:
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, unsubscribe.Unsubscribe()) })

	requireFired(t, fired, "adapter powered subscription did not receive firehose signal")
}
