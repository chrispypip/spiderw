//go:build unit

package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

func fakeWithAdapter() *fakeClient {
	ad := &fakeAdapter{
		path: "/net/connman/iwd/phy0",
		props: &spiderw.AdapterProperties{
			Powered:        true,
			Name:           "phy0",
			Model:          new("MockModel"),
			Vendor:         new("MockVendor"),
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

func TestAdapterCmd_ScopedStatus_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, true, "adapter", "phy0", "status")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)

	entry := list[0]
	require.Equal(t, "/net/connman/iwd/phy0", entry["Path"])
	require.Equal(t, "phy0", entry["Name"])
	require.Equal(t, true, entry["Powered"])
	require.Equal(t, "MockModel", entry["Model"])
	require.Equal(t, "MockVendor", entry["Vendor"])
	require.Equal(t, []any{"station", "ap"}, entry["SupportedModes"])
}

func TestAdapterCmd_ScopedStatus_UsageError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "status", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw adapter <adapter> status")
	require.NotContains(t, out, "Commands:")
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

func TestAdapterCmd_List(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "phy0")
}

func TestAdapterCmd_ScalarAccessors(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		t.Parallel()
		out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "name")
		require.Equal(t, 0, code, out)
		require.Equal(t, "phy0\n", out)
	})

	t.Run("Model", func(t *testing.T) {
		t.Parallel()
		out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "model")
		require.Equal(t, 0, code, out)
		require.Equal(t, "MockModel\n", out)
	})

	t.Run("Vendor", func(t *testing.T) {
		t.Parallel()
		out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "vendor")
		require.Equal(t, 0, code, out)
		require.Equal(t, "MockVendor\n", out)
	})

	t.Run("SupportedModes", func(t *testing.T) {
		t.Parallel()
		out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "supported-modes")
		require.Equal(t, 0, code, out)
		require.Contains(t, out, "station")
		require.Contains(t, out, "ap")
	})
}

func TestAdapterCmd_ScalarAccessors_BackendError(t *testing.T) {
	t.Parallel()

	newFailing := func() *fakeClient {
		ad := &fakeAdapter{path: "/net/connman/iwd/phy0", err: errors.New("backend boom")}
		return &fakeClient{
			daemon:   &fakeDaemon{adapters: []spiderw.AdapterRef{{Path: ad.path, Name: "phy0"}}},
			adapters: map[string]adapterAPI{ad.path: ad},
		}
	}

	for _, sub := range []string{"name", "model", "vendor", "supported-modes"} {
		t.Run(sub, func(t *testing.T) {
			t.Parallel()
			out, code := driveCLI(newFailing(), nil, false, "adapter", "phy0", sub)
			require.Equal(t, 1, code, out)
			require.Contains(t, out, "backend boom")
		})
	}
}

func TestAdapterCmd_InvalidModeArg(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "supports-mode", "42")
	require.Equal(t, 1, code)
	require.Contains(t, out, "invalid mode")
}

func TestAdapterCmd_MissingNestedCommandPrintsHelp(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0")
	require.Equal(t, 1, code)
	require.Contains(t, out, "Usage:")
	require.Contains(t, out, "spiderw adapter <command>")
	require.Contains(t, out, "Commands:")
	require.Contains(t, out, "<adapter> powered")
	require.Contains(t, out, "missing adapter command for \"phy0\"")
}

func TestAdapterCmd_UnknownNestedCommandPrintsHelp(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "bogus")
	require.Equal(t, 1, code)
	require.Contains(t, out, "Usage:")
	require.Contains(t, out, "Commands:")
	require.Contains(t, out, "supported-modes")
	require.Contains(t, out, "unknown adapter command \"bogus\" for adapter \"phy0\"")
}

func TestAdapterCmd_LeafUsageErrorStaysFocused(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", "powered", "on", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw adapter <adapter> powered [true|false]")
	require.NotContains(t, out, "Commands:")
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
			t.Parallel()
			got, err := parseModeArg(in)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}

	for _, in := range []string{"", "42", "monitor", "p2p"} {
		t.Run("invalid/"+in, func(t *testing.T) {
			t.Parallel()
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

	require.Equal(t, "Intel", adapterOptionalStringResult{Value: new("Intel")}.String())
	require.Empty(t, adapterOptionalStringResult{Value: nil}.String())

	require.Equal(t, "station\nap", adapterSupportedModesResult{SupportedModes: []string{"station", "ap"}}.String())
}

func TestAdapterStatusResult_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no adapters available", adapterStatusResult{}.String())

	out := adapterStatusResult{
		{
			Path:           "/net/connman/iwd/phy0",
			Name:           "phy0",
			Powered:        true,
			Model:          new("MockModel"),
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

// TestPrintAdapterPoweredLine covers the monitor output helper directly (the
// monitor command itself blocks on an OS signal and is not drivable in-process).
func TestPrintAdapterPoweredLine(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex

	app, buf := appWithBuffer(false)
	require.NoError(t, printAdapterPoweredLine(app, "phy0", true, &mu))
	require.Equal(t, "powered=true\n", buf.String())

	appJSON, bufJSON := appWithBuffer(true)
	require.NoError(t, printAdapterPoweredLine(appJSON, "phy0", false, &mu))
	require.Contains(t, bufJSON.String(), `"Powered":false`)
	require.Contains(t, bufJSON.String(), `"phy0"`)
}

// TestAdapterCmd_SupportsAPAndAdHoc covers the two capability subcommands that had
// no test; supports-station and supports-mode were covered, these were not.
func TestAdapterCmd_SupportsAPAndAdHoc(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		sub  string
		want string
	}{
		{"supports-ap", "true"},
		{"supports-adhoc", "false"},
		{"supports-ad-hoc", "false"},
	} {
		t.Run(tc.sub, func(t *testing.T) {
			t.Parallel()
			out, code := driveCLI(fakeWithAdapter(), nil, false, "adapter", "phy0", tc.sub)
			require.Equal(t, 0, code, out)
			require.Contains(t, out, tc.want)
		})
	}
}
