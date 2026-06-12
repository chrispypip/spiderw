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

func TestSignalFirehose_DeviceSurvives(t *testing.T) {
	iwdmock.StartMockFirehose(t)

	for range 20 {
		out, err := runSpiderDevice(t, "list")
		require.NoError(t, err, "device list failed under firehose load: %s", out)
		require.Contains(t, out, "wlan0")
		require.Contains(t, out, devicePath)

		out, err = runSpiderDevice(t, "wlan0", "powered")
		require.NoError(t, err, "device powered failed under firehose load: %s", out)
		require.Contains(t, []string{"true\n", "false\n"}, out)

		out, err = runSpiderDevice(t, "wlan0", "mode")
		require.NoError(t, err, "device mode failed under firehose load: %s", out)
		require.Contains(t, []string{"station\n", "ap\n", "ad-hoc\n"}, out)
	}
}

func TestSignalFirehose_PublicDeviceSubscribeReceivesSignals(t *testing.T) {
	iwdmock.StartMockFirehose(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)
	device := newPublicMockDevice(t, ctx, client, "wlan0")

	poweredFired := make(chan struct{}, 10)
	modeFired := make(chan struct{}, 10)

	unsubPowered, err := device.SubscribePoweredChanged(ctx, func(bool) {
		select {
		case poweredFired <- struct{}{}:
		default:
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, unsubPowered.Unsubscribe()) })

	unsubMode, err := device.SubscribeModeChanged(ctx, func(spiderw.Mode) {
		select {
		case modeFired <- struct{}{}:
		default:
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, unsubMode.Unsubscribe()) })

	requireFired(t, poweredFired, "device powered subscription did not receive firehose signal")
	requireFired(t, modeFired, "device mode subscription did not receive firehose signal")
}
