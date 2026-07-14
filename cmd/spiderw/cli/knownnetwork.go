package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/chrispypip/spiderw"
)

type knownNetworkRefResult struct {
	Path string `json:"Path"`
	Name string `json:"Name"`
}

type knownNetworkListResult []knownNetworkRefResult

// String returns the CLI string form of the value.
func (r knownNetworkListResult) String() string {
	if len(r) == 0 {
		return "no known networks available"
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

// parseBoolArg parses a CLI boolean argument, accepting common true/false words.
func parseBoolArg(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "y", "on", "enable", "enabled":
		return true, nil
	case "false", "0", "no", "n", "off", "disable", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", raw)
	}
}

func knownNetworkRefs(ctx context.Context, client clientAPI) ([]spiderw.KnownNetworkRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.KnownNetworks(ctx)
}

func runKnownNetworkList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown known-network list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := knownNetworkRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(knownNetworkListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, knownNetworkRefResult{Path: ref.Path, Name: ref.Name})
	}
	return app.printOutput(out)
}

type knownNetworkStatusEntry struct {
	Path              string  `json:"Path"`
	Name              string  `json:"Name"`
	Type              string  `json:"Type"`
	Hidden            bool    `json:"Hidden"`
	LastConnectedTime *string `json:"LastConnectedTime"`
	AutoConnect       bool    `json:"AutoConnect"`
}

type knownNetworkStatusResult []knownNetworkStatusEntry

// String returns the CLI string form of the value.
func (r knownNetworkStatusResult) String() string {
	if len(r) == 0 {
		return "no known networks available"
	}

	value := func(v string) string {
		if v == "" {
			return "-"
		}
		return v
	}
	field := func(label, value string) string {
		return fmt.Sprintf("%-20s%s", label+":", value)
	}

	blocks := make([]string, 0, len(r))
	for _, entry := range r {
		name := entry.Name
		if name == "" {
			name = "(unnamed)"
		}

		lastConnected := "-"
		if entry.LastConnectedTime != nil {
			lastConnected = *entry.LastConnectedTime
		}

		lines := []string{
			field("Name", name),
			field("Path", entry.Path),
			field("Type", value(entry.Type)),
			field("Hidden", fmt.Sprintf("%t", entry.Hidden)),
			field("LastConnectedTime", lastConnected),
			field("AutoConnect", fmt.Sprintf("%t", entry.AutoConnect)),
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func knownNetworkStatusEntryFrom(ctx context.Context, k knownNetworkAPI) (knownNetworkStatusEntry, error) {
	// One Properties.GetAll call per known network instead of one Get per property.
	props, err := k.Properties(ctx)
	if err != nil {
		return knownNetworkStatusEntry{}, err
	}

	return knownNetworkStatusEntry{
		Path:              k.Path(),
		Name:              props.Name,
		Type:              props.Type.String(),
		Hidden:            props.Hidden,
		LastConnectedTime: props.LastConnectedTime,
		AutoConnect:       props.AutoConnect,
	}, nil
}

func runKnownNetworkStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown known-network status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	known, err := client.AllKnownNetworks(ctx)
	if err != nil {
		return err
	}

	out := make(knownNetworkStatusResult, 0, len(known))
	for _, k := range known {
		entry, err := knownNetworkStatusEntryFrom(ctx, k)
		if err != nil {
			return err
		}

		out = append(out, entry)
	}
	return app.printOutput(out)
}

func runKnownNetworkSingleStatus(app *App, ctx context.Context, ref string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw known-network <known-network> status")
	}

	return withKnownNetwork(app, ctx, ref, func(ctx context.Context, k knownNetworkAPI) error {
		entry, err := knownNetworkStatusEntryFrom(ctx, k)
		if err != nil {
			return err
		}

		return app.printOutput(knownNetworkStatusResult{entry})
	})
}

func knownNetworkByRef(ctx context.Context, client clientAPI, ref string) (knownNetworkAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("known-network reference required")
	}

	refs, err := knownNetworkRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no known networks available")
	}

	var matches []spiderw.KnownNetworkRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Name == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("known network %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("known-network reference %q is ambiguous; use a path", ref)
	}

	return client.KnownNetwork(ctx, matches[0].Path)
}

func withKnownNetwork(app *App, ctx context.Context, ref string, fn func(ctx context.Context, k knownNetworkAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		k, err := knownNetworkByRef(ctx, client, ref)
		if err != nil {
			return err
		}

		return fn(ctx, k)
	})
}

type knownNetworkAutoConnectResult struct {
	KnownNetwork string `json:"KnownNetwork"`
	AutoConnect  bool   `json:"AutoConnect"`
}

// String returns the CLI string form of the value.
func (r knownNetworkAutoConnectResult) String() string {
	return fmt.Sprintf("%t", r.AutoConnect)
}

type knownNetworkStringResult struct {
	KnownNetwork string `json:"KnownNetwork"`
	Value        string `json:"Value"`
}

// String returns the CLI string form of the value.
func (r knownNetworkStringResult) String() string {
	return r.Value
}

type knownNetworkForgetResult struct {
	KnownNetwork string `json:"KnownNetwork"`
	Forgotten    bool   `json:"Forgotten"`
}

// String returns the CLI string form of the value.
func (r knownNetworkForgetResult) String() string {
	return fmt.Sprintf("forgotten %s", r.KnownNetwork)
}

func runKnownNetworkAutoConnect(app *App, ctx context.Context, ref string, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: spiderw known-network <known-network> autoconnect [true|false]")
	}

	return withKnownNetwork(app, ctx, ref, func(ctx context.Context, k knownNetworkAPI) error {
		if len(args) == 0 {
			auto, err := k.AutoConnect(ctx)
			if err != nil {
				return err
			}
			return app.printOutput(knownNetworkAutoConnectResult{KnownNetwork: ref, AutoConnect: auto})
		}

		auto, err := parseBoolArg(args[0])
		if err != nil {
			return fmt.Errorf("invalid value for autoconnect: %q (expected true|false)", args[0])
		}

		if err := k.SetAutoConnect(ctx, auto); err != nil {
			return err
		}

		newVal, err := k.AutoConnect(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(knownNetworkAutoConnectResult{KnownNetwork: ref, AutoConnect: newVal})
	})
}

func runKnownNetworkForget(app *App, ctx context.Context, ref string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw known-network <known-network> forget")
	}

	return withKnownNetwork(app, ctx, ref, func(ctx context.Context, k knownNetworkAPI) error {
		if err := k.Forget(ctx); err != nil {
			return err
		}
		return app.printOutput(knownNetworkForgetResult{KnownNetwork: ref, Forgotten: true})
	})
}

func runKnownNetworkHidden(app *App, ctx context.Context, ref string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw known-network <known-network> hidden")
	}

	return withKnownNetwork(app, ctx, ref, func(ctx context.Context, k knownNetworkAPI) error {
		hidden, err := k.Hidden(ctx)
		if err != nil {
			return err
		}
		return app.printOutput(knownNetworkStringResult{KnownNetwork: ref, Value: fmt.Sprintf("%t", hidden)})
	})
}

func runKnownNetworkLastConnected(app *App, ctx context.Context, ref string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw known-network <known-network> last-connected")
	}

	return withKnownNetwork(app, ctx, ref, func(ctx context.Context, k knownNetworkAPI) error {
		lt, err := k.LastConnectedTime(ctx)
		if err != nil {
			return err
		}
		value := ""
		if lt != nil {
			value = *lt
		}
		return app.printOutput(knownNetworkStringResult{KnownNetwork: ref, Value: value})
	})
}

func runKnownNetworkString(app *App, ctx context.Context, ref string, args []string, usage string, op func(ctx context.Context, k knownNetworkAPI) (string, error)) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}

	return withKnownNetwork(app, ctx, ref, func(ctx context.Context, k knownNetworkAPI) error {
		value, err := op(ctx, k)
		if err != nil {
			return err
		}
		return app.printOutput(knownNetworkStringResult{KnownNetwork: ref, Value: value})
	})
}

func printKnownNetworkAutoConnectLine(app *App, ref string, auto bool, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := knownNetworkAutoConnectResult{KnownNetwork: ref, AutoConnect: auto}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "autoconnect=%t\n", auto)
	return err
}

// knownNetworkHiddenResult reports the known network's Hidden property.
type knownNetworkHiddenResult struct {
	KnownNetwork string `json:"KnownNetwork"`
	Hidden       bool   `json:"Hidden"`
}

func printKnownNetworkHiddenLine(app *App, ref string, hidden bool, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := knownNetworkHiddenResult{KnownNetwork: ref, Hidden: hidden}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "hidden=%t\n", hidden)
	return err
}

// knownNetworkLastConnectedResult reports the known network's LastConnectedTime. A
// nil value means the network has never been connected to.
type knownNetworkLastConnectedResult struct {
	KnownNetwork      string  `json:"KnownNetwork"`
	LastConnectedTime *string `json:"LastConnectedTime"`
}

func printKnownNetworkLastConnectedLine(app *App, ref string, ts *string, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := knownNetworkLastConnectedResult{KnownNetwork: ref, LastConnectedTime: ts}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "last-connected=%s\n", optionalPathText(ts, "never"))
	return err
}

const knownNetworkMonitorUsage = "usage: spiderw known-network <known-network> monitor <autoconnect|hidden|last-connected>"

// knownNetworkMonitorTargets are the properties the monitor subcommand streams.
var knownNetworkMonitorTargets = []string{"autoconnect", "hidden", "last-connected"}

// parseKnownNetworkMonitorTarget validates the target before any iwd call.
func parseKnownNetworkMonitorTarget(args []string) (string, error) {
	// `monitor --help` should list what can be monitored, same as an invalid
	// target does, rather than falling through to a generic error.
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		return "", fmt.Errorf("%s", knownNetworkMonitorUsage)
	}
	if len(args) != 1 || !slices.Contains(knownNetworkMonitorTargets, args[0]) {
		return "", fmt.Errorf("%s", knownNetworkMonitorUsage)
	}
	return args[0], nil
}

// streamKnownNetworkProperty prints the current value, then subscribes. It does
// not block; monitorKnownNetwork owns the wait for Ctrl-C.
func streamKnownNetworkProperty(ctx context.Context, app *App, ref, what string, k knownNetworkAPI, mu *sync.Mutex) (spiderw.UnsubscribeFunc, error) {
	switch what {
	case "autoconnect":
		auto, err := k.AutoConnect(ctx)
		if err != nil {
			return nil, err
		}
		if err := printKnownNetworkAutoConnectLine(app, ref, auto, mu); err != nil {
			return nil, err
		}
		return k.SubscribeAutoConnectChanged(ctx, func(auto bool) {
			_ = printKnownNetworkAutoConnectLine(app, ref, auto, mu)
		})

	case "hidden":
		hidden, err := k.Hidden(ctx)
		if err != nil {
			return nil, err
		}
		if err := printKnownNetworkHiddenLine(app, ref, hidden, mu); err != nil {
			return nil, err
		}
		return k.SubscribeHiddenChanged(ctx, func(hidden bool) {
			_ = printKnownNetworkHiddenLine(app, ref, hidden, mu)
		})

	case "last-connected":
		ts, err := k.LastConnectedTime(ctx)
		if err != nil {
			return nil, err
		}
		if err := printKnownNetworkLastConnectedLine(app, ref, ts, mu); err != nil {
			return nil, err
		}
		// iwd rewrites the timestamp on each successful connection, so this line
		// reprints once per connect to this network.
		return k.SubscribeLastConnectedTimeChanged(ctx, func(ts *string) {
			_ = printKnownNetworkLastConnectedLine(app, ref, ts, mu)
		})
	}

	return nil, fmt.Errorf("%s", knownNetworkMonitorUsage)
}

func monitorKnownNetwork(app *App, ref string, args []string) error {
	what, err := parseKnownNetworkMonitorTarget(args)
	if err != nil {
		return err
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

	k, err := knownNetworkByRef(ctx, client, ref)
	if err != nil {
		return err
	}

	var printMu sync.Mutex
	unsubscribe, err := streamKnownNetworkProperty(ctx, app, ref, what, k, &printMu)
	if err != nil {
		return err
	}
	defer func() {
		_ = unsubscribe.Unsubscribe()
	}()

	<-ctx.Done()
	return nil
}

func runKnownNetworkWithRef(app *App, args []string) error {
	if len(args) < 2 {
		printKnownNetworkUsage(app)
		return fmt.Errorf("missing known-network command for %q", args[0])
	}

	ref := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "status":
		return runKnownNetworkSingleStatus(app, ctx, ref, rest)
	case "name":
		return runKnownNetworkString(app, ctx, ref, rest, "spiderw known-network <known-network> name", func(ctx context.Context, k knownNetworkAPI) (string, error) {
			return k.Name(ctx)
		})
	case "type":
		return runKnownNetworkString(app, ctx, ref, rest, "spiderw known-network <known-network> type", func(ctx context.Context, k knownNetworkAPI) (string, error) {
			t, err := k.Type(ctx)
			if err != nil {
				return "", err
			}
			return t.String(), nil
		})
	case "hidden":
		return runKnownNetworkHidden(app, ctx, ref, rest)
	case "last-connected":
		return runKnownNetworkLastConnected(app, ctx, ref, rest)
	case "autoconnect":
		return runKnownNetworkAutoConnect(app, ctx, ref, rest)
	case "forget":
		return runKnownNetworkForget(app, ctx, ref, rest)
	case "monitor":
		return monitorKnownNetwork(app, ref, rest)
	default:
		printKnownNetworkUsage(app)
		return fmt.Errorf("unknown known-network command %q for known network %q", op, ref)
	}
}

func runKnownNetwork(app *App, args []string) error {
	if len(args) == 0 {
		printKnownNetworkUsage(app)
		return fmt.Errorf("missing known-network command")
	}

	switch args[0] {
	case "list":
		return runKnownNetworkList(app, args[1:])
	case "status":
		return runKnownNetworkStatus(app, args[1:])
	}

	return runKnownNetworkWithRef(app, args)
}

const knownNetworkHelpText = `Commands:
  list                                       List known networks (name and path)
  status                                     Show a snapshot of every known network
  <known-network> status                     Show a snapshot of one known network
  <known-network> name                       Show the known network name
  <known-network> type                       Show the known network type
  <known-network> hidden                     Show whether the network is hidden
  <known-network> last-connected             Show the last-connected timestamp, if any
  <known-network> autoconnect [true|false]   Get or set auto-connect
  <known-network> forget                     Forget (remove) the known network
  <known-network> monitor autoconnect  Stream the auto-connect flag until Ctrl-C
  <known-network> monitor hidden       Stream the hidden flag. This reports how
                                      the profile was provisioned (connected to
                                      by SSID because it was not broadcasting),
                                      not whether the AP is hiding its SSID now
  <known-network> monitor last-connected
                                      Stream the last-connected timestamp; fires
                                      on each successful connect`

func knownNetworkCommand(app *App) *Command {
	return &Command{
		Name:        "known-network",
		Description: "Inspect and manage iwd known networks",
		HelpText:    knownNetworkHelpText,
		SubUsage:    map[string]string{"monitor": knownNetworkMonitorUsage},
		Execute: func(args []string) error {
			return runKnownNetwork(app, args)
		},
	}
}

func printKnownNetworkUsage(app *App) {
	knownNetworkCommand(app).printUsage(app)
}
