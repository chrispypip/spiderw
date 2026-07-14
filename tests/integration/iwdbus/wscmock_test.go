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

// TestSimpleConfigurationMock_CLI drives `station <ref> wsc ...` in-process
// against the mock. This is the only coverage of the runStationWSC ->
// SimpleConfiguration -> runWSCOp seam: the unit tests drive runWSCOp directly
// with a fake handle (a real *spiderw.SimpleConfiguration cannot be built over a
// fake backend), so without this the wiring from a typed command line to the
// enrollment call is never executed anywhere.
func TestSimpleConfigurationMock_CLI(t *testing.T) {
	t.Run("push-button", func(t *testing.T) {
		iwdmock.StartMockNormal(t)

		out, err := runSpider(t, "station", devicePath, "wsc", "push-button")
		require.NoError(t, err, out)
		// The prompt precedes the blocking enrollment, then the result.
		mustContainAll(t, out, []string{"press the WPS button", "connected via WSC"})
	})

	t.Run("pin generated", func(t *testing.T) {
		iwdmock.StartMockNormal(t)

		// With no PIN argument the CLI generates one, prints it, and enrolls with it.
		out, err := runSpider(t, "station", devicePath, "wsc", "pin")
		require.NoError(t, err, out)
		mustContainAll(t, out, []string{"12345670", "connected via WSC"})
	})

	t.Run("pin supplied", func(t *testing.T) {
		iwdmock.StartMockNormal(t)

		out, err := runSpider(t, "station", devicePath, "wsc", "pin", "12345670")
		require.NoError(t, err, out)
		mustContainAll(t, out, []string{"12345670", "connected via WSC"})
	})

	t.Run("cancel", func(t *testing.T) {
		iwdmock.StartMockNormal(t)

		out, err := runSpider(t, "station", devicePath, "wsc", "cancel")
		require.NoError(t, err, out)
		mustContain(t, out, "canceled")
	})

	t.Run("enrollment failure surfaces", func(t *testing.T) {
		iwdmock.StartMockNormal(t)

		// The sentinel PIN drives iwd's WSC NoCredentials error; the CLI must fail
		// rather than reporting a connection.
		out, err := runSpider(t, "station", devicePath, "wsc", "pin", "00000000")
		require.Error(t, err, out)
		require.NotContains(t, out, "connected via WSC")
	})
}

// TestSimpleConfigurationMock_CLI_Unavailable drives `station <ref> wsc` against a
// device with no SimpleConfiguration interface, confirming the CLI surfaces the
// missing handle instead of panicking.
func TestSimpleConfigurationMock_CLI_Unavailable(t *testing.T) {
	iwdmock.StartMockWithoutWSC(t)

	out, err := runSpider(t, "station", devicePath, "wsc", "push-button")
	require.Error(t, err, out)
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
