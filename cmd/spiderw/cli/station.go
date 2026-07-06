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
	Name string `json:"Name"`
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
		if ref.Name != "" {
			fmt.Fprintf(&b, "%s\t%s", ref.Name, ref.Path)
		} else {
			b.WriteString(ref.Path)
		}
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
		out = append(out, stationRefResult{Path: ref.Path, Name: ref.Name})
	}
	return app.printOutput(out)
}

type stationStatusEntry struct {
	Path                 string    `json:"Path"`
	Name                 string    `json:"Name"`
	State                string    `json:"State"`
	Scanning             bool      `json:"Scanning"`
	ConnectedNetwork     *nameRef  `json:"ConnectedNetwork"`
	ConnectedAccessPoint *addrRef  `json:"ConnectedAccessPoint"`
	Affinities           []addrRef `json:"Affinities"`
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
	optNameRef := func(r *nameRef) string {
		if r == nil {
			return "-"
		}
		return r.readable()
	}
	optAddrRef := func(r *addrRef) string {
		if r == nil {
			return "-"
		}
		return r.readable()
	}
	field := func(label, value string) string {
		return fmt.Sprintf("%-22s%s", label+":", value)
	}

	blocks := make([]string, 0, len(r))
	for _, entry := range r {
		lines := []string{
			field("Path", entry.Path),
			field("Name", value(entry.Name)),
			field("State", value(entry.State)),
			field("Scanning", fmt.Sprintf("%t", entry.Scanning)),
			field("ConnectedNetwork", optNameRef(entry.ConnectedNetwork)),
			field("ConnectedAccessPoint", optAddrRef(entry.ConnectedAccessPoint)),
			field("Affinities", readableAddrRefs(entry.Affinities)),
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
		Name:       s.Name(),
		State:      props.State.String(),
		Scanning:   props.Scanning,
		Affinities: toAddrRefs(props.Affinities),
	}
	if props.ConnectedNetwork != nil {
		cn := toNameRef(props.ConnectedNetwork.Name, props.ConnectedNetwork.Path)
		entry.ConnectedNetwork = &cn
	}
	if props.ConnectedAccessPoint != nil {
		ap := toAddrRef(props.ConnectedAccessPoint.Address, props.ConnectedAccessPoint.Path)
		entry.ConnectedAccessPoint = &ap
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

	// A station has no Name of its own; it is referenced by path or by the name
	// of the device it shares an object with (e.g. "wlan0").
	refs, err := stationRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no stations available")
	}

	var matches []spiderw.StationRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Name == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("station %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("station reference %q is ambiguous; use a station path", ref)
	}
	return client.Station(ctx, matches[0].Path)
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
	timeout := fs.Duration("timeout", scanWaitTimeout, "how long to wait for the scan to finish (wait mode)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: spiderw station <station> scan [--no-wait] [--timeout=<duration>]")
	}

	timeoutSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "timeout" {
			timeoutSet = true
		}
	})
	if *noWait && timeoutSet {
		return fmt.Errorf("--timeout has no effect with --no-wait")
	}
	if *timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
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
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(*timeout):
			return fmt.Errorf("timed out waiting for scan to finish after %s", *timeout)
		}

		return printStationNetworks(app, ctx, s)
	})
}

type stationNetworkResult struct {
	Name      string  `json:"Name"`
	Path      string  `json:"Path"`
	SignalDBm float64 `json:"SignalDBm"`
}

// readable returns the network's SSID, falling back to its path when unresolved.
func (r stationNetworkResult) readable() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Path
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
		fmt.Fprintf(&b, "%s\t%g dBm", n.readable(), n.SignalDBm)
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
		out = append(out, stationNetworkResult{Name: n.Name, Path: n.Path, SignalDBm: n.SignalStrength})
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

type stationAffinitiesResult []addrRef

// String returns the CLI string form of the value: one BSS MAC (or path when
// unresolved) per line.
func (r stationAffinitiesResult) String() string {
	if len(r) == 0 {
		return "no affinities set"
	}
	lines := make([]string, 0, len(r))
	for _, ref := range r {
		lines = append(lines, ref.readable())
	}
	return strings.Join(lines, "\n")
}

func runStationAffinities(app *App, ctx context.Context, stationRef string, args []string) error {
	// `affinities` shows the current list; `affinities set <bss>...` writes it
	// (each <bss> a MAC or object path); `affinities clear` removes them all.
	if len(args) > 0 && args[0] == "set" {
		return runStationAffinitiesSet(app, ctx, stationRef, args[1:])
	}
	if len(args) > 0 && args[0] == "clear" {
		if len(args) != 1 {
			return fmt.Errorf("usage: spiderw station <station> affinities clear")
		}
		return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
			if err := s.SetAffinities(ctx, nil); err != nil {
				return err
			}
			return app.printOutput(stationAffinitiesResult(nil))
		})
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw station <station> affinities [set <bss>... | clear]")
	}

	// Read via Properties so affinities render as resolved BSS MACs.
	return withStation(app, ctx, stationRef, func(ctx context.Context, s stationAPI) error {
		props, err := s.Properties(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(stationAffinitiesResult(toAddrRefs(props.Affinities)))
	})
}

func runStationAffinitiesSet(app *App, ctx context.Context, stationRef string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: spiderw station <station> affinities set <bss> [<bss>...] (BSS MAC or object path)")
	}
	return app.withClient(ctx, func(client clientAPI) error {
		s, err := stationByRef(ctx, client, stationRef)
		if err != nil {
			return err
		}
		refs, err := resolveAffinityRefs(ctx, client, args)
		if err != nil {
			return err
		}
		paths := make([]string, len(refs))
		for i, r := range refs {
			paths[i] = r.Path
		}
		if err := s.SetAffinities(ctx, paths); err != nil {
			return err
		}
		return app.printOutput(stationAffinitiesResult(refs))
	})
}

// resolveAffinityRefs turns affinity arguments (each a BSS MAC or a full object
// path) into resolved refs. A MAC is matched device-wide against every BSS's
// Address; a value starting with "/" is taken as a path verbatim.
func resolveAffinityRefs(ctx context.Context, client clientAPI, args []string) ([]addrRef, error) {
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}

	var bsses []spiderw.BasicServiceSetRef
	fetched := false
	fetch := func() error {
		if fetched {
			return nil
		}
		b, err := daemon.BasicServiceSets(ctx)
		if err != nil {
			return err
		}
		bsses, fetched = b, true
		return nil
	}

	out := make([]addrRef, 0, len(args))
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			return nil, fmt.Errorf("empty affinity reference")
		}
		if err := fetch(); err != nil {
			return nil, err
		}
		if strings.HasPrefix(a, "/") {
			addr := ""
			for _, b := range bsses {
				if b.Path == a {
					addr = b.Address
					break
				}
			}
			out = append(out, addrRef{Address: addr, Path: a})
			continue
		}
		found := ""
		for _, b := range bsses {
			if strings.EqualFold(b.Address, a) {
				found = b.Path
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("no basic service set found with address %q", a)
		}
		out = append(out, addrRef{Address: a, Path: found})
	}
	return out, nil
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
  <station> scan [--no-wait] [--timeout=<dur>]
                                        Scan for networks (waits for completion,
                                        then lists results, unless --no-wait;
                                        --timeout bounds the wait, default 15s)
  <station> networks                    List networks from the last scan
  <station> disconnect                  Disconnect from the current network
  <station> connect-hidden <ssid> [--passphrase=<s> | --passphrase-stdin]
                                        Connect to a hidden network by SSID
  <station> hidden-aps                  List hidden access points from the scan
  <station> affinities                  Show the station's affinity BSSes (MACs)
  <station> affinities set <bss>...     Set affinities by BSS MAC or object path
  <station> affinities clear            Remove all affinities

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
