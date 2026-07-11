//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// TestSimpleConfigurationMock_RoundTrip drives WSC enrollment end to end against
// the mock over the real session bus: PushButton, GeneratePin + StartPin, and
// Cancel all succeed.
func TestSimpleConfigurationMock_RoundTrip(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	wsc, err := station.SimpleConfiguration(ctx)
	require.NoError(t, err)
	require.NotNil(t, wsc)

	// PushButton (PBC) enrollment succeeds.
	require.NoError(t, wsc.PushButton(ctx))

	// GeneratePin returns the mock's fixed PIN, and StartPin with it succeeds.
	pin, err := wsc.GeneratePin(ctx)
	require.NoError(t, err)
	require.Equal(t, "12345670", pin)
	require.NoError(t, wsc.StartPin(ctx, pin))

	require.NoError(t, wsc.Cancel(ctx))
}

// TestSimpleConfigurationMock_StartPinNoCredentials verifies the WSC error
// taxonomy surfaces end to end: the sentinel PIN drives iwd's WSC NoCredentials
// error, which must remain matchable via the public sentinel after normalization
// and wrapping through every layer.
func TestSimpleConfigurationMock_StartPinNoCredentials(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	wsc, err := station.SimpleConfiguration(ctx)
	require.NoError(t, err)

	err = wsc.StartPin(ctx, "00000000")
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrWSCNoCredentials)
}

// TestSimpleConfigurationMock_Unavailable verifies that when the station-mode
// device does not expose the SimpleConfiguration interface (--omit-wsc, like a
// driver without WSC support), the station still works but Station.SimpleConfiguration
// fails cleanly rather than hanging or panicking.
func TestSimpleConfigurationMock_Unavailable(t *testing.T) {
	iwdmock.StartMockWithoutWSC(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)
	require.NotNil(t, station)

	// The station itself is fully functional; only WSC was omitted.
	_, err = station.Properties(ctx)
	require.NoError(t, err)

	wsc, err := station.SimpleConfiguration(ctx)
	require.Error(t, err)
	require.Nil(t, wsc)
}
