package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/chrispypip/spiderw"
)

type networkRefResult struct {
	Path string `json:"Path"`
	Name string `json:"Name"`
}

type networkListResult []networkRefResult

// String returns the CLI string form of the value.
func (r networkListResult) String() string {
	if len(r) == 0 {
		return "no networks available"
	}

	var b strings.Builder
	for i, ref := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		if ref.Name == "" {
			b.WriteString(ref.Path)
			continue
		}
		fmt.Fprintf(&b, "%s\t%s", ref.Name, ref.Path)
	}
	return b.String()
}

func networkRefs(ctx context.Context, client clientAPI) ([]spiderw.NetworkRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.Networks(ctx)
}

func runNetworkList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown network list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := networkRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(networkListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, networkRefResult{Path: ref.Path, Name: ref.Name})
	}
	return app.printOutput(out)
}

type networkStatusEntry struct {
	Path               string   `json:"Path"`
	Name               string   `json:"Name"`
	Connected          bool     `json:"Connected"`
	Type               string   `json:"Type"`
	Device             string   `json:"Device"`
	KnownNetwork       *string  `json:"KnownNetwork"`
	ExtendedServiceSet []string `json:"ExtendedServiceSet"`
}

type networkStatusResult []networkStatusEntry

// String returns the CLI string form of the value.
func (r networkStatusResult) String() string {
	if len(r) == 0 {
		return "no networks available"
	}

	value := func(v string) string {
		if v == "" {
			return "-"
		}
		return v
	}
	field := func(label, value string) string {
		return fmt.Sprintf("%-16s%s", label+":", value)
	}

	blocks := make([]string, 0, len(r))
	for _, entry := range r {
		name := entry.Name
		if name == "" {
			name = "(unnamed)"
		}

		known := "-"
		if entry.KnownNetwork != nil {
			known = *entry.KnownNetwork
		}

		ess := "-"
		if len(entry.ExtendedServiceSet) > 0 {
			ess = strings.Join(entry.ExtendedServiceSet, ", ")
		}

		lines := []string{
			field("Name", name),
			field("Path", entry.Path),
			field("Connected", fmt.Sprintf("%t", entry.Connected)),
			field("Type", value(entry.Type)),
			field("Device", value(entry.Device)),
			field("KnownNetwork", known),
			field("BasicServiceSets", ess),
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func networkStatusEntryFromNetwork(ctx context.Context, n networkAPI) (networkStatusEntry, error) {
	// One Properties.GetAll call per network instead of one Get per property.
	props, err := n.Properties(ctx)
	if err != nil {
		return networkStatusEntry{}, err
	}

	return networkStatusEntry{
		Path:               n.Path(),
		Name:               props.Name,
		Connected:          props.Connected,
		Type:               props.Type.String(),
		Device:             props.Device,
		KnownNetwork:       props.KnownNetwork,
		ExtendedServiceSet: props.ExtendedServiceSet,
	}, nil
}

func runNetworkStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown network status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	networks, err := client.AllNetworks(ctx)
	if err != nil {
		return err
	}

	out := make(networkStatusResult, 0, len(networks))
	for _, n := range networks {
		entry, err := networkStatusEntryFromNetwork(ctx, n)
		if err != nil {
			return err
		}

		out = append(out, entry)
	}
	return app.printOutput(out)
}

func runNetworkSingleStatus(app *App, ctx context.Context, networkRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw network <network> status")
	}

	return withNetwork(app, ctx, networkRef, func(ctx context.Context, n networkAPI) error {
		entry, err := networkStatusEntryFromNetwork(ctx, n)
		if err != nil {
			return err
		}

		return app.printOutput(networkStatusResult{entry})
	})
}

func networkByRef(ctx context.Context, client clientAPI, ref string) (networkAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("network reference required")
	}

	refs, err := networkRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no networks available")
	}

	var matches []spiderw.NetworkRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Name == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("network %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("network reference %q is ambiguous; use a network path", ref)
	}

	return client.Network(ctx, matches[0].Path)
}

func withNetwork(app *App, ctx context.Context, networkRef string, fn func(context.Context, networkAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		n, err := networkByRef(ctx, client, networkRef)
		if err != nil {
			return err
		}

		return fn(ctx, n)
	})
}

type networkConnectedResult struct {
	Network   string `json:"Network"`
	Connected bool   `json:"Connected"`
}

// String returns the CLI string form of the value.
func (r networkConnectedResult) String() string {
	return fmt.Sprintf("%t", r.Connected)
}

type networkStringResult struct {
	Network string `json:"Network"`
	Value   string `json:"Value"`
}

// String returns the CLI string form of the value.
func (r networkStringResult) String() string {
	return r.Value
}

type networkBSSesResult struct {
	Network            string   `json:"Network"`
	ExtendedServiceSet []string `json:"ExtendedServiceSet"`
}

// String returns the CLI string form of the value.
func (r networkBSSesResult) String() string {
	if len(r.ExtendedServiceSet) == 0 {
		return "no basic service sets available"
	}
	return strings.Join(r.ExtendedServiceSet, "\n")
}

func runNetworkConnect(app *App, ctx context.Context, networkRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw network <network> connect")
	}

	return withNetwork(app, ctx, networkRef, func(ctx context.Context, n networkAPI) error {
		if err := n.Connect(ctx); err != nil {
			return err
		}

		connected, err := n.Connected(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(networkConnectedResult{Network: networkRef, Connected: connected})
	})
}

func runNetworkConnected(app *App, ctx context.Context, networkRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw network <network> connected")
	}

	return withNetwork(app, ctx, networkRef, func(ctx context.Context, n networkAPI) error {
		connected, err := n.Connected(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(networkConnectedResult{Network: networkRef, Connected: connected})
	})
}

func runNetworkString(app *App, ctx context.Context, networkRef string, args []string, usage string, op func(context.Context, networkAPI) (string, error)) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}

	return withNetwork(app, ctx, networkRef, func(ctx context.Context, n networkAPI) error {
		value, err := op(ctx, n)
		if err != nil {
			return err
		}

		return app.printOutput(networkStringResult{Network: networkRef, Value: value})
	})
}

func runNetworkKnownNetwork(app *App, ctx context.Context, networkRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw network <network> known-network")
	}

	return withNetwork(app, ctx, networkRef, func(ctx context.Context, n networkAPI) error {
		known, err := n.KnownNetwork(ctx)
		if err != nil {
			return err
		}
		value := ""
		if known != nil {
			value = *known
		}
		return app.printOutput(networkStringResult{Network: networkRef, Value: value})
	})
}

func runNetworkBSSes(app *App, ctx context.Context, networkRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw network <network> bsses")
	}

	return withNetwork(app, ctx, networkRef, func(ctx context.Context, n networkAPI) error {
		ess, err := n.ExtendedServiceSet(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(networkBSSesResult{Network: networkRef, ExtendedServiceSet: ess})
	})
}

func printNetworkConnectedLine(app *App, ref string, connected bool, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := networkConnectedResult{Network: ref, Connected: connected}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "connected=%t\n", connected)
	return err
}

func monitorNetwork(app *App, networkRef string, args []string) error {
	if len(args) != 1 || args[0] != "connected" {
		return fmt.Errorf("usage: spiderw network <network> monitor connected")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	n, err := networkByRef(ctx, client, networkRef)
	if err != nil {
		return err
	}

	var printMu sync.Mutex

	connected, err := n.Connected(ctx)
	if err != nil {
		return err
	}
	if err := printNetworkConnectedLine(app, networkRef, connected, &printMu); err != nil {
		return err
	}

	unsubscribe, err := n.SubscribeConnectedChanged(ctx, func(connected bool) {
		_ = printNetworkConnectedLine(app, networkRef, connected, &printMu)
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = unsubscribe.Unsubscribe()
	}()

	<-ctx.Done()
	return nil
}

func runNetworkWithRef(app *App, args []string) error {
	if len(args) < 2 {
		printNetworkUsage(app)
		return fmt.Errorf("missing network command for %q", args[0])
	}

	networkRef := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "status":
		return runNetworkSingleStatus(app, ctx, networkRef, rest)
	case "connect":
		return runNetworkConnect(app, ctx, networkRef, rest)
	case "connected":
		return runNetworkConnected(app, ctx, networkRef, rest)
	case "type":
		return runNetworkString(app, ctx, networkRef, rest, "spiderw network <network> type", func(ctx context.Context, n networkAPI) (string, error) {
			t, err := n.Type(ctx)
			if err != nil {
				return "", err
			}
			return t.String(), nil
		})
	case "device":
		return runNetworkString(app, ctx, networkRef, rest, "spiderw network <network> device", func(ctx context.Context, n networkAPI) (string, error) {
			return n.Device(ctx)
		})
	case "name":
		return runNetworkString(app, ctx, networkRef, rest, "spiderw network <network> name", func(ctx context.Context, n networkAPI) (string, error) {
			return n.Name(ctx)
		})
	case "known-network":
		return runNetworkKnownNetwork(app, ctx, networkRef, rest)
	case "bsses":
		return runNetworkBSSes(app, ctx, networkRef, rest)
	case "monitor":
		return monitorNetwork(app, networkRef, rest)
	default:
		printNetworkUsage(app)
		return fmt.Errorf("unknown network command %q for network %q", op, networkRef)
	}
}

func runNetwork(app *App, args []string) error {
	if len(args) == 0 {
		printNetworkUsage(app)
		return fmt.Errorf("missing network command")
	}

	switch args[0] {
	case "list":
		return runNetworkList(app, args[1:])
	case "status":
		return runNetworkStatus(app, args[1:])
	}

	return runNetworkWithRef(app, args)
}

const networkHelpText = `Commands:
  list                             List networks (name and path)
  status                           Show a snapshot of every network
  <network> status                 Show a snapshot of one network
  <network> connect                Connect to the network (open/known only without an agent)
  <network> connected              Show whether the network is connected
  <network> type                   Show the network security type
  <network> device                 Show the owning device object path
  <network> name                   Show the network SSID
  <network> known-network          Show the known-network object path, if any
  <network> bsses                  List the network's basic service set paths
  <network> monitor connected      Stream connected-state changes`

func networkCommand(app *App) *Command {
	return &Command{
		Name:        "network",
		Description: "Inspect, query, and connect to iwd networks",
		HelpText:    networkHelpText,
		Execute: func(args []string) error {
			return runNetwork(app, args)
		},
	}
}

func printNetworkUsage(app *App) {
	networkCommand(app).printUsage(app)
}
