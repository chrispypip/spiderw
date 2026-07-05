package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/chrispypip/spiderw"
)

// scanWaitTimeout bounds how long `station <path> scan` (wait mode) blocks for a
// scan to finish before giving up.
const scanWaitTimeout = 15 * time.Second

type stationRefResult struct {
	Path string `json:"Path"`
}

type stationListResult []stationRefResult

// String returns the CLI string form of the value.
func (r stationListResult) String() string {
	if len(r) == 0 {
		return "no stations available"
	}

	var b strings.Builder
	for i, ref := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ref.Path)
	}
	return b.String()
}

func stationRefs(ctx context.Context, client clientAPI) ([]spiderw.StationRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.Stations(ctx)
}

func runStationList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown station list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := stationRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(stationListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, stationRefResult{Path: ref.Path})
	}
	return app.printOutput(out)
}

type stationStatusEntry struct {
	Path                 string   `json:"Path"`
	State                string   `json:"State"`
	Scanning             bool     `json:"Scanning"`
	ConnectedNetwork     string   `json:"ConnectedNetwork"`
	ConnectedAccessPoint string   `json:"ConnectedAccessPoint"`
	Affinities           []string `json:"Affinities"`
}

type stationStatusResult []stationStatusEntry

// String returns the CLI string form of the value.
func (r stationStatusResult) String() string {
	if len(r) == 0 {
		return "no stations available"
	}

	value := func(v string) string {
		if v == "" {
			return "-"
		}
		return v
	}
	list := func(v []string) string {
		if len(v) == 0 {
			return "-"
		}
		return strings.Join(v, ", ")
	}
	field := func(label, value string) string {
		return fmt.Sprintf("%-22s%s", label+":", value)
	}

	blocks := make([]string, 0, len(r))
	for _, entry := range r {
		lines := []string{
			field("Path", entry.Path),
			field("State", value(entry.State)),
			field("Scanning", fmt.Sprintf("%t", entry.Scanning)),
			field("ConnectedNetwork", value(entry.ConnectedNetwork)),
			field("ConnectedAccessPoint", value(entry.ConnectedAccessPoint)),
			field("Affinities", list(entry.Affinities)),
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func stationStatusEntryFromStation(ctx context.Context, s stationAPI) (stationStatusEntry, error) {
	// One Properties.GetAll call per station instead of one Get per property.
	props, err := s.Properties(ctx)
	if err != nil {
		return stationStatusEntry{}, err
	}

	entry := stationStatusEntry{
		Path:       s.Path(),
		State:      props.State.String(),
		Scanning:   props.Scanning,
		Affinities: props.Affinities,
	}
	if props.ConnectedNetwork != nil {
		entry.ConnectedNetwork = *props.ConnectedNetwork
	}
	if props.ConnectedAccessPoint != nil {
		entry.ConnectedAccessPoint = *props.ConnectedAccessPoint
	}
	return entry, nil
}

func runStationStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown station status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	stations, err := client.AllStations(ctx)
	if err != nil {
		return err
	}

	out := make(stationStatusResult, 0, len(stations))
	for _, s := range stations {
		entry, err := stationStatusEntryFromStation(ctx, s)
		if err != nil {
			return err
		}
		out = append(out, entry)
	}
	return app.printOutput(out)
}

func runStationSingleStatus(app *App, ctx context.Context, stationRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw station <station> status")
	}

	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		entry, err := stationStatusEntryFromStation(ctx, s)
		if err != nil {
			return err
		}
		return app.printOutput(stationStatusResult{entry})
	})
}

func stationByRef(ctx context.Context, client clientAPI, ref string) (stationAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("station reference required")
	}

	// A station has no Name of its own (it shares the device object), so it is
	// referenced by path only.
	refs, err := stationRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no stations available")
	}

	for _, candidate := range refs {
		if candidate.Path == ref {
			return client.Station(ctx, candidate.Path)
		}
	}
	return nil, fmt.Errorf("station %q not found", ref)
}

func withStation(app *App, ctx context.Context, stationRef string, fn func(ctx context.Context, s stationAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		s, err := stationByRef(ctx, client, stationRef)
		if err != nil {
			return err
		}
		return fn(ctx, s)
	})
}

type stationScanResult struct {
	Station string `json:"Station"`
	Started bool   `json:"Started"`
}

// String returns the CLI string form of the value.
func (r stationScanResult) String() string {
	return "scan started"
}

func runStationScan(app *App, ctx context.Context, stationRef string, args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(app.stderr())
	noWait := fs.Bool("no-wait", false, "trigger the scan and return without waiting for it to finish")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: spiderw station <station> scan [--no-wait]")
	}

	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		if *noWait {
			if err := s.Scan(ctx); err != nil {
				return err
			}
			return app.printOutput(stationScanResult{Station: stationRef, Started: true})
		}

		// Wait: subscribe to Scanning, start the scan, then block until Scanning
		// returns to false. Subscribing before Scan avoids missing the transition.
		done := make(chan struct{}, 1)
		unsubscribe, err := s.SubscribeScanningChanged(ctx, func(scanning bool) {
			if !scanning {
				select {
				case done <- struct{}{}:
				default:
				}
			}
		})
		if err != nil {
			return err
		}
		defer func() {
			_ = unsubscribe.Unsubscribe()
		}()

		if err := s.Scan(ctx); err != nil {
			return err
		}

		select {
		case <-done:
		case <-time.After(scanWaitTimeout):
			return fmt.Errorf("timed out waiting for scan to finish")
		}

		return printStationNetworks(app, ctx, s)
	})
}

type stationNetworkResult struct {
	Network   string  `json:"Network"`
	SignalDBm float64 `json:"SignalDBm"`
}

type stationNetworksResult []stationNetworkResult

// String returns the CLI string form of the value.
func (r stationNetworksResult) String() string {
	if len(r) == 0 {
		return "no networks available"
	}
	var b strings.Builder
	for i, n := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s\t%g dBm", n.Network, n.SignalDBm)
	}
	return b.String()
}

func printStationNetworks(app *App, ctx context.Context, s stationAPI) error {
	nets, err := s.OrderedNetworks(ctx)
	if err != nil {
		return err
	}
	out := make(stationNetworksResult, 0, len(nets))
	for _, n := range nets {
		out = append(out, stationNetworkResult{Network: n.Network, SignalDBm: n.SignalStrength})
	}
	return app.printOutput(out)
}

func runStationNetworks(app *App, ctx context.Context, stationRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw station <station> networks")
	}
	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		return printStationNetworks(app, ctx, s)
	})
}

type stationAffinitiesResult []string

// String returns the CLI string form of the value.
func (r stationAffinitiesResult) String() string {
	if len(r) == 0 {
		return "no affinities set"
	}
	return strings.Join(r, "\n")
}

func runStationAffinities(app *App, ctx context.Context, stationRef string, args []string) error {
	// `affinities` shows the current list; `affinities set <bss-path>...` writes it.
	if len(args) > 0 && args[0] == "set" {
		paths := args[1:]
		if len(paths) == 0 {
			return fmt.Errorf("usage: spiderw station <station> affinities set <bss-path> [<bss-path>...]")
		}
		return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
			if err := s.SetAffinities(ctx, paths); err != nil {
				return err
			}
			return app.printOutput(stationAffinitiesResult(paths))
		})
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw station <station> affinities [set <bss-path>...]")
	}

	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		affinities, err := s.Affinities(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(stationAffinitiesResult(affinities))
	})
}

type stationDisconnectResult struct {
	Station string `json:"Station"`
}

// String returns the CLI string form of the value.
func (r stationDisconnectResult) String() string { return "disconnected" }

func runStationDisconnect(app *App, ctx context.Context, stationRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw station <station> disconnect")
	}
	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		if err := s.Disconnect(ctx); err != nil {
			return err
		}
		return app.printOutput(stationDisconnectResult{Station: stationRef})
	})
}

type stationConnectHiddenResult struct {
	Station   string `json:"Station"`
	Network   string `json:"Network"`
	Connected bool   `json:"Connected"`
}

// String returns the CLI string form of the value.
func (r stationConnectHiddenResult) String() string {
	return fmt.Sprintf("connected to %s", r.Network)
}

const stationConnectHiddenUsage = "usage: spiderw station <station> connect-hidden <ssid> [--passphrase=<secret> | --passphrase-stdin]"

func runStationConnectHidden(app *App, ctx context.Context, stationRef string, args []string) error {
	// The SSID is the first positional; flags follow it (args[1:]).
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("%s", stationConnectHiddenUsage)
	}
	ssid := args[0]

	fs := flag.NewFlagSet("connect-hidden", flag.ContinueOnError)
	fs.SetOutput(app.stderr())
	passphrase := fs.String("passphrase", "", "passphrase for a secured (PSK) hidden network")
	passStdin := fs.Bool("passphrase-stdin", false, "read the passphrase from the first line of stdin")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("%s", stationConnectHiddenUsage)
	}

	passphraseSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "passphrase" {
			passphraseSet = true
		}
	})
	if passphraseSet && *passStdin {
		return fmt.Errorf("specify only one of --passphrase or --passphrase-stdin")
	}

	return app.withClient(ctx, func(client clientAPI) error {
		s, err := stationByRef(ctx, client, stationRef)
		if err != nil {
			return err
		}

		// The network is hidden, so security can't be checked up front. Register an
		// agent whose passphrase callback resolves the secret lazily: iwd invokes it
		// only for a secured hidden network, so open ones never prompt.
		agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
			Passphrase: func(ctx context.Context, networkPath string) (string, error) {
				return resolveConnectPassphrase(app, ssid, passphraseSet, *passphrase, *passStdin)
			},
		})
		if err != nil {
			return err
		}
		defer func() { _ = agent.Unregister(context.Background()) }()

		if err := s.ConnectHiddenNetwork(ctx, ssid); err != nil {
			return err
		}
		return app.printOutput(stationConnectHiddenResult{Station: stationRef, Network: ssid, Connected: true})
	})
}

type stationHiddenAPResult struct {
	Address   string  `json:"Address"`
	SignalDBm float64 `json:"SignalDBm"`
	Type      string  `json:"Type"`
}

type stationHiddenAPsResult []stationHiddenAPResult

// String returns the CLI string form of the value.
func (r stationHiddenAPsResult) String() string {
	if len(r) == 0 {
		return "no hidden access points available"
	}
	var b strings.Builder
	for i, ap := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s\t%g dBm\t%s", ap.Address, ap.SignalDBm, ap.Type)
	}
	return b.String()
}

func runStationHiddenAPs(app *App, ctx context.Context, stationRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw station <station> hidden-aps")
	}
	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		aps, err := s.HiddenAccessPoints(ctx)
		if err != nil {
			return err
		}
		out := make(stationHiddenAPsResult, 0, len(aps))
		for _, ap := range aps {
			out = append(out, stationHiddenAPResult{Address: ap.Address, SignalDBm: ap.SignalStrength, Type: ap.Type.String()})
		}
		return app.printOutput(out)
	})
}

func runStationWithRef(app *App, args []string) error {
	if len(args) < 2 {
		printStationUsage(app)
		return fmt.Errorf("missing station command for %q", args[0])
	}

	stationRef := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "status":
		return runStationSingleStatus(app, ctx, stationRef, rest)
	case "scan":
		return runStationScan(app, ctx, stationRef, rest)
	case "networks":
		return runStationNetworks(app, ctx, stationRef, rest)
	case "affinities":
		return runStationAffinities(app, ctx, stationRef, rest)
	case "disconnect":
		return runStationDisconnect(app, ctx, stationRef, rest)
	case "connect-hidden":
		return runStationConnectHidden(app, ctx, stationRef, rest)
	case "hidden-aps":
		return runStationHiddenAPs(app, ctx, stationRef, rest)
	default:
		printStationUsage(app)
		return fmt.Errorf("unknown station command %q for station %q", op, stationRef)
	}
}

func runStation(app *App, args []string) error {
	if len(args) == 0 {
		printStationUsage(app)
		return fmt.Errorf("missing station command")
	}

	switch args[0] {
	case "list":
		return runStationList(app, args[1:])
	case "status":
		return runStationStatus(app, args[1:])
	}

	return runStationWithRef(app, args)
}

const stationHelpText = `Commands:
  list                                  List stations (object paths)
  status                                Show a snapshot of every station
  <station> status                      Show a snapshot of one station (by path)
  <station> scan [--no-wait]            Scan for networks (waits for completion,
                                        then lists results, unless --no-wait)
  <station> networks                    List networks from the last scan
  <station> disconnect                  Disconnect from the current network
  <station> connect-hidden <ssid> [--passphrase=<s> | --passphrase-stdin]
                                        Connect to a hidden network by SSID
  <station> hidden-aps                  List hidden access points from the scan
  <station> affinities                  Show the station's affinity BSS paths
  <station> affinities set <p>...       Set the station's affinity BSS paths

A station is a device in station mode. Connecting to a *visible* network is done
via 'network <ssid> connect'.`

func stationCommand(app *App) *Command {
	return &Command{
		Name:        "station",
		Description: "Inspect iwd station (station-mode device) connection state",
		HelpText:    stationHelpText,
		Execute: func(args []string) error {
			return runStation(app, args)
		},
	}
}

func printStationUsage(app *App) {
	stationCommand(app).printUsage(app)
}
