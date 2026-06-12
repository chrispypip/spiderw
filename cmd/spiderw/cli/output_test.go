//go:build unit

package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// appWithBuffer returns an App whose stdout is captured, in either JSON or
// human mode.
func appWithBuffer(jsonOut bool) (*App, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &App{Stdout: buf, Output: outputConfig{JSON: jsonOut}}, buf
}

func TestPrintOutput_Human_UsesStringer(t *testing.T) {
	t.Parallel()

	app, buf := appWithBuffer(false)
	require.NoError(t, app.printOutput(devicePoweredResult{Device: "wlan0", Powered: false}))
	// Human mode renders the Stringer form (not a Go struct literal) + newline.
	require.Equal(t, "false\n", buf.String())
}

func TestPrintOutput_JSON_ScalarShape(t *testing.T) {
	t.Parallel()

	app, buf := appWithBuffer(true)
	require.NoError(t, app.printOutput(devicePoweredResult{Device: "wlan0", Powered: true}))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "wlan0", got["Device"])
	require.Equal(t, true, got["Powered"])
}

func TestPrintOutput_JSON_DeviceStatusShape(t *testing.T) {
	t.Parallel()

	app, buf := appWithBuffer(true)
	err := app.printOutput(deviceStatusResult{
		{
			Path:    "/net/connman/iwd/phy0/wlan0",
			Name:    "wlan0",
			Address: "aa:bb:cc:dd:ee:ff",
			Powered: true,
			Mode:    "station",
			Adapter: "/net/connman/iwd/phy0",
		},
	})
	require.NoError(t, err)

	// `device status` emits a JSON array of per-device snapshots.
	var got []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got, 1)

	entry := got[0]
	require.Equal(t, "/net/connman/iwd/phy0/wlan0", entry["Path"])
	require.Equal(t, "wlan0", entry["Name"])
	require.Equal(t, "aa:bb:cc:dd:ee:ff", entry["Address"])
	require.Equal(t, true, entry["Powered"])
	require.Equal(t, "station", entry["Mode"])
	require.Equal(t, "/net/connman/iwd/phy0", entry["Adapter"])
}

func TestPrintOutput_JSON_AdapterStatusNullOptionals(t *testing.T) {
	t.Parallel()

	app, buf := appWithBuffer(true)
	err := app.printOutput(adapterStatusResult{
		{Path: "/net/connman/iwd/phy0", Name: "phy0", Powered: true, Model: nil, Vendor: nil},
	})
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got, 1)

	entry := got[0]
	require.Equal(t, "phy0", entry["Name"])
	// Absent optionals serialize as JSON null (the key is present with nil value).
	require.Contains(t, entry, "Model")
	require.Nil(t, entry["Model"])
	require.Contains(t, entry, "Vendor")
	require.Nil(t, entry["Vendor"])
}
