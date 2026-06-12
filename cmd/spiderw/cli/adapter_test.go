//go:build unit

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

func fakeWithAdapter() *fakeClient {
	model := "MockModel"
	vendor := "MockVendor"
	ad := &fakeAdapter{
		path: "/net/connman/iwd/phy0",
		props: &spiderw.AdapterProperties{
			Powered:        true,
			Name:           "phy0",
			Model:          &model,
			Vendor:         &vendor,
			SupportedModes: []spiderw.Mode{spiderw.ModeStation, spiderw.ModeAP},
		},
	}
	return &fakeClient{
		daemon:      &fakeDaemon{adapters: []spiderw.AdapterRef{{Path: ad.path, Name: "phy0"}}},
		adapters:    map[string]adapterAPI{ad.path: ad},
		allAdapters: []adapterAPI{ad},
	}
}

func TestAdapterCmd_Status_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "status")
	require.Equal(t, 0, code, out)
	for _, want := range []string{"phy0", "MockModel", "MockVendor", "station, ap", "/net/connman/iwd/phy0"} {
		require.Contains(t, out, want)
	}
}

func TestAdapterCmd_PoweredGetAndSet(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "powered")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true\n", out)

	out, code = driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "powered", "off")
	require.Equal(t, 0, code, out)
	require.Equal(t, "false\n", out)
}

func TestAdapterCmd_SupportsMode(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "supports-station")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true\n", out)

	out, code = driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "supports-ad-hoc")
	require.Equal(t, 0, code, out)
	require.Equal(t, "false\n", out)
}

func TestAdapterCmd_InvalidModeArg(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "supports-mode", "42")
	require.Equal(t, 1, code)
	require.Contains(t, out, "invalid mode")
}

func TestAdapterCmd_RefNotFound(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(&fakeClient{daemon: &fakeDaemon{}}, nil, false, "adapter", "phy0", "powered")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no adapters available")
}

func TestAdapterCmd_BackendError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(&fakeClient{allAdaptErr: errors.New("enumeration boom")}, nil, false, "adapter", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}

func TestAdapterCmd_ClientConstructionError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(nil, errors.New("no session bus"), false, "adapter", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no session bus")
}

func TestParseModeArg(t *testing.T) {
	t.Parallel()

	valid := map[string]spiderw.Mode{
		"station":      spiderw.ModeStation,
		"sta":          spiderw.ModeStation,
		"STATION":      spiderw.ModeStation,
		"  station  ":  spiderw.ModeStation,
		"ap":           spiderw.ModeAP,
		"access-point": spiderw.ModeAP,
		"accesspoint":  spiderw.ModeAP,
		"ad-hoc":       spiderw.ModeAdHoc,
		"adhoc":        spiderw.ModeAdHoc,
		"ad_hoc":       spiderw.ModeAdHoc,
		"ibss":         spiderw.ModeAdHoc,
	}
	for in, want := range valid {
		t.Run("valid/"+in, func(t *testing.T) {
			got, err := parseModeArg(in)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}

	for _, in := range []string{"", "42", "monitor", "p2p"} {
		t.Run("invalid/"+in, func(t *testing.T) {
			got, err := parseModeArg(in)
			require.Error(t, err)
			require.Equal(t, spiderw.ModeUnknown, got)
			require.Contains(t, err.Error(), "invalid mode")
		})
	}
}

func TestAdapterListResult_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no adapters available", adapterListResult{}.String())

	out := adapterListResult{
		{Path: "/net/connman/iwd/phy0", Name: "phy0"},
		{Path: "/net/connman/iwd/phy1", Name: ""},
	}.String()
	require.Equal(t, "phy0\t/net/connman/iwd/phy0\n/net/connman/iwd/phy1", out)
}

func TestAdapterScalarResults_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "true", adapterPoweredResult{Powered: true}.String())
	require.Equal(t, "false", adapterPoweredResult{Powered: false}.String())
	require.Equal(t, "true", adapterBoolResult{Value: true}.String())
	require.Equal(t, "phy0", adapterStringResult{Value: "phy0"}.String())

	model := "Intel"
	require.Equal(t, "Intel", adapterOptionalStringResult{Value: &model}.String())
	require.Equal(t, "", adapterOptionalStringResult{Value: nil}.String())

	require.Equal(t, "station\nap", adapterSupportedModesResult{SupportedModes: []string{"station", "ap"}}.String())
}

func TestAdapterStatusResult_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no adapters available", adapterStatusResult{}.String())

	model := "MockModel"
	out := adapterStatusResult{
		{
			Path:           "/net/connman/iwd/phy0",
			Name:           "phy0",
			Powered:        true,
			Model:          &model,
			Vendor:         nil, // absent optional renders as "-"
			SupportedModes: []string{"station", "ap"},
		},
	}.String()

	require.Contains(t, out, "Name:")
	require.Contains(t, out, "phy0")
	require.Contains(t, out, "Path:")
	require.Contains(t, out, "/net/connman/iwd/phy0")
	require.Contains(t, out, "Powered:")
	require.Contains(t, out, "true")
	require.Contains(t, out, "Model:")
	require.Contains(t, out, "MockModel")
	require.Contains(t, out, "Vendor:")
	require.Contains(t, out, "SupportedModes:")
	require.Contains(t, out, "station, ap")
	// Absent Vendor renders as "-", not an empty value or "<nil>".
	require.Contains(t, out, "Vendor:")
	require.NotContains(t, out, "<nil>")
}

func TestAdapterStatusResult_String_MultipleEntriesSeparated(t *testing.T) {
	t.Parallel()

	out := adapterStatusResult{
		{Path: "/p0", Name: "phy0"},
		{Path: "/p1", Name: "phy1"},
	}.String()

	// Distinct adapters are separated by a blank line.
	require.Equal(t, 2, strings.Count(out, "Name:"))
	require.Contains(t, out, "\n\n")
}

func TestAdapterStatusResult_String_UnnamedAndEmptyModes(t *testing.T) {
	t.Parallel()

	out := adapterStatusResult{
		{Path: "/p0", Name: "", SupportedModes: nil},
	}.String()

	require.Contains(t, out, "(unnamed)")
	// No modes renders as "-".
	require.Contains(t, out, "SupportedModes:")
	require.Contains(t, out, "-")
}
