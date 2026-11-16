package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
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
	Path               string    `json:"Path"`
	Name               string    `json:"Name"`
	Connected          bool      `json:"Connected"`
	Type               string    `json:"Type"`
	Device             nameRef   `json:"Device"`
	KnownNetwork       *string   `json:"KnownNetwork"`
	ExtendedServiceSet []addrRef `json:"ExtendedServiceSet"`
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

		lines := []string{
			field("Name", name),
			field("Path", entry.Path),
			field("Connected", fmt.Sprintf("%t", entry.Connected)),
			field("Type", value(entry.Type)),
			field("Device", entry.Device.readable()),
			field("KnownNetwork", known),
			field("BasicServiceSets", readableAddrRefs(entry.ExtendedServiceSet)),
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

	entry := networkStatusEntry{
		Path:               n.Path(),
		Name:               props.Name,
		Connected:          props.Connected,
		Type:               props.Type.String(),
		Device:             toNameRef(props.Device.Name, props.Device.Path),
		KnownNetwork:       props.KnownNetwork,
		ExtendedServiceSet: toAddrRefs(props.ExtendedServiceSet),
	}
	return entry, nil
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

func withNetwork(app *App, ctx context.Context, networkRef string, fn func(ctx context.Context, n networkAPI) error) error {
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

const networkConnectUsage = "usage: spiderw network <network> connect " +
	"[--passphrase=<secret> | --passphrase-stdin] " +
	"[--username=<id>] [--password=<secret> | --password-stdin] " +
	"[--private-key-passphrase=<secret> | --private-key-passphrase-stdin]"

func runNetworkConnect(app *App, ctx context.Context, networkRef string, args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(app.stderr())
	passphrase := fs.String("passphrase", "", "passphrase for a secured (PSK) network")
	passStdin := fs.Bool("passphrase-stdin", false, "read the passphrase from the first line of stdin")
	username := fs.String("username", "", "EAP identity for an 802.1x network")
	password := fs.String("password", "", "EAP password for an 802.1x network")
	passwordStdin := fs.Bool("password-stdin", false, "read the 802.1x password from the first line of stdin")
	privKey := fs.String("private-key-passphrase", "", "EAP-TLS private-key passphrase for an 802.1x network")
	privKeyStdin := fs.Bool("private-key-passphrase-stdin", false, "read the private-key passphrase from the first line of stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("%s", networkConnectUsage)
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["passphrase"] && *passStdin {
		return fmt.Errorf("specify only one of --passphrase or --passphrase-stdin")
	}
	if set["password"] && *passwordStdin {
		return fmt.Errorf("specify only one of --password or --password-stdin")
	}
	if set["private-key-passphrase"] && *privKeyStdin {
		return fmt.Errorf("specify only one of --private-key-passphrase or --private-key-passphrase-stdin")
	}
	flags := connectFlags{
		set:           set,
		passphrase:    *passphrase,
		passStdin:     *passStdin,
		username:      *username,
		password:      *password,
		passwordStdin: *passwordStdin,
		privKey:       *privKey,
		privKeyStdin:  *privKeyStdin,
	}

	return app.withClient(ctx, func(client clientAPI) error {
		n, err := networkByRef(ctx, client, networkRef)
		if err != nil {
			return err
		}

		needsAgent, netType, err := networkNeedsAgent(ctx, n)
		if err != nil {
			return err
		}

		// Register an agent when iwd will ask us for credentials: always for a
		// not-yet-known secured network, and also for a KNOWN secured network when
		// the user supplied credentials. A provisioned 802.1x network is "known"
		// via its .8021x file yet can still omit the secret and ask the agent at
		// connect time, so "known" does not imply "needs no agent" for 802.1x.
		if needsAgent || (netType != spiderw.NetworkTypeOpen && flags.hasCredential()) {
			cfg, err := buildConnectAgentConfig(app, networkRef, netType, flags)
			if err != nil {
				return err
			}
			agent, err := client.RegisterAgent(ctx, cfg)
			if err != nil {
				return err
			}
			defer func() { _ = agent.Unregister(context.Background()) }()
		}

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

// networkNeedsAgent reports whether connecting to n DEFINITELY requires a
// credentials agent - true for a secured network iwd does not already know;
// false for an open network or one iwd knows - and the network's security Type,
// so the caller can pick the right credentials (PSK passphrase vs 802.1x username
// and password).
//
// A "known" result is not the whole story: a provisioned 802.1x network is known
// via its .8021x file yet can still ask the agent for a secret the file omits, so
// the caller ALSO registers an agent for a known secured network when the user
// supplied credentials (see hasCredential).
//
// It reads Type and KnownNetwork in a single Properties (GetAll) call, which also
// avoids a single-property Get of the optional KnownNetwork (absent for a network
// iwd does not know).
func networkNeedsAgent(ctx context.Context, n networkAPI) (bool, spiderw.NetworkType, error) {
	props, err := n.Properties(ctx)
	if err != nil {
		return false, spiderw.NetworkTypeUnknown, err
	}
	if props.Type == spiderw.NetworkTypeOpen {
		return false, props.Type, nil
	}
	return props.KnownNetwork == nil, props.Type, nil
}

// connectFlags holds the parsed credential flags for a network connect. Which
// callbacks they wire depends on the network's security type, so they are
// gathered here and resolved in buildConnectAgentConfig.
type connectFlags struct {
	set           map[string]bool
	passphrase    string
	passStdin     bool
	username      string
	password      string
	passwordStdin bool
	privKey       string
	privKeyStdin  bool
}

// hasCredential reports whether the user supplied any credential on the command
// line, as opposed to relying on iwd's stored profile or an interactive prompt.
// It decides whether to register an agent for a network iwd already knows: a
// provisioned 802.1x network can be "known" yet still need the agent's secret.
func (f connectFlags) hasCredential() bool {
	return f.set["passphrase"] || f.passStdin ||
		f.set["username"] || f.set["password"] || f.passwordStdin ||
		f.set["private-key-passphrase"] || f.privKeyStdin
}

// buildConnectAgentConfig resolves the credentials for a secured connect and
// returns the AgentConfig to register, chosen by the network's security type:
// PSK networks use --passphrase, 802.1x networks use --username/--password (EAP
// username+password) and/or --private-key-passphrase (EAP-TLS). Mixing a PSK
// flag with an 802.1x one is rejected so the error names the right flags.
func buildConnectAgentConfig(app *App, networkRef string, netType spiderw.NetworkType, f connectFlags) (spiderw.AgentConfig, error) {
	if netType == spiderw.NetworkType8021x {
		return build8021xAgentConfig(app, networkRef, f)
	}

	if f.set["username"] || f.set["password"] || f.passwordStdin ||
		f.set["private-key-passphrase"] || f.privKeyStdin {
		return spiderw.AgentConfig{}, fmt.Errorf(
			"%q is not an 802.1x network; use --passphrase, not the 802.1x flags", networkRef)
	}
	secret, err := resolveConnectPassphrase(app, networkRef, f.set["passphrase"], f.passphrase, f.passStdin)
	if err != nil {
		return spiderw.AgentConfig{}, err
	}
	return spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) {
			return secret, nil
		},
	}, nil
}

// build8021xAgentConfig wires the EAP credential callbacks for an 802.1x connect
// from the flags supplied: --password (optionally with --username) for the EAP
// username/password methods, and --private-key-passphrase for EAP-TLS. When no
// 802.1x credential is given at all it falls back to prompting for a password
// (the common PEAP/TTLS case).
func build8021xAgentConfig(app *App, networkRef string, f connectFlags) (spiderw.AgentConfig, error) {
	if f.set["passphrase"] || f.passStdin {
		return spiderw.AgentConfig{}, fmt.Errorf(
			"%q is an 802.1x network; use --username/--password or --private-key-passphrase, "+
				"not --passphrase", networkRef)
	}

	cfg := spiderw.AgentConfig{}

	pkProvided := f.set["private-key-passphrase"] || f.privKeyStdin
	if pkProvided {
		pass, err := resolveSecret(app, f.set["private-key-passphrase"], f.privKey, f.privKeyStdin,
			fmt.Sprintf("Private-key passphrase for %s: ", networkRef))
		if err != nil {
			return spiderw.AgentConfig{}, err
		}
		// iwd calls this for EAP-TLS when the client's private key is encrypted.
		cfg.PrivateKeyPassphrase = func(ctx context.Context, networkPath string) (string, error) {
			return pass, nil
		}
	}

	// Password path (EAP username/password). Also the fallback that prompts when
	// no 802.1x credential was supplied, preserving the interactive default.
	if f.set["password"] || f.passwordStdin || !pkProvided {
		pass, err := resolveSecret(app, f.set["password"], f.password, f.passwordStdin,
			fmt.Sprintf("Password for %s: ", networkRef))
		if err != nil {
			return spiderw.AgentConfig{}, err
		}
		// iwd calls this when the network's own config already carries the identity.
		cfg.UserPassword = func(ctx context.Context, networkPath, iwdUser string) (string, error) {
			return pass, nil
		}
		if f.username != "" {
			// iwd calls this when the config has no identity and it needs both.
			cfg.UserNameAndPassword = func(ctx context.Context, networkPath string) (string, string, error) {
				return f.username, pass, nil
			}
		}
	}

	return cfg, nil
}

// resolveSecret obtains a secret in precedence order: its flag, then its -stdin
// variant, then an interactive no-echo prompt with promptMsg.
func resolveSecret(app *App, flagSet bool, flagVal string, stdin bool, promptMsg string) (string, error) {
	switch {
	case flagSet:
		return flagVal, nil
	case stdin:
		return readPassphraseStdin(app)
	default:
		return app.promptPassphrase(promptMsg)
	}
}

// resolveConnectPassphrase obtains the passphrase for a secured connect, in
// precedence order: --passphrase, then --passphrase-stdin, then an interactive
// no-echo terminal prompt.
func resolveConnectPassphrase(app *App, networkRef string, passphraseSet bool, passphrase string, passStdin bool) (string, error) {
	switch {
	case passphraseSet:
		return passphrase, nil
	case passStdin:
		return readPassphraseStdin(app)
	default:
		return app.promptPassphrase(fmt.Sprintf("Passphrase for %s: ", networkRef))
	}
}

func readPassphraseStdin(app *App) (string, error) {
	scanner := bufio.NewScanner(app.stdin())
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading passphrase from stdin: %w", err)
		}
		return "", fmt.Errorf("no passphrase provided on stdin")
	}
	return scanner.Text(), nil
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

func runNetworkString(app *App, ctx context.Context, networkRef string, args []string, usage string, op func(ctx context.Context, n networkAPI) (string, error)) error {
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

// networkKnownNetworkResult reports the network's known-network association,
// resolved to its name. A nil ref means the network is not saved.
type networkKnownNetworkResult struct {
	Network      string   `json:"Network"`
	KnownNetwork *nameRef `json:"KnownNetwork"`
}

func printNetworkKnownNetworkLine(app *App, ref string, kn *nameRef, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := networkKnownNetworkResult{Network: ref, KnownNetwork: kn}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "known-network=%s\n", readableNameRef(kn, "none (not saved)"))
	return err
}

func printNetworkBSSesLine(app *App, ref string, paths []string, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	// Reuse the `bsses` subcommand's result shape so the monitor stream and the
	// one-shot read render identically.
	out := networkBSSesResult{Network: ref, ExtendedServiceSet: paths}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "bsses=%s\n", pathListText(paths))
	return err
}

const networkMonitorUsage = "usage: spiderw network <network> monitor <connected|known-network|bsses>"

// networkMonitorTargets are the properties `network <network> monitor` streams.
var networkMonitorTargets = []string{"connected", "known-network", "bsses"}

// parseNetworkMonitorTarget validates the target before any iwd call.
func parseNetworkMonitorTarget(args []string) (string, error) {
	// `monitor --help` should list what can be monitored, same as an invalid
	// target does, rather than falling through to a generic error.
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		return "", fmt.Errorf("%s", networkMonitorUsage)
	}
	if len(args) != 1 || !slices.Contains(networkMonitorTargets, args[0]) {
		return "", fmt.Errorf("%s", networkMonitorUsage)
	}
	return args[0], nil
}

// streamNetworkProperty prints the current value, then subscribes. It does not
// block; monitorNetwork owns the wait for Ctrl-C.
func streamNetworkProperty(ctx context.Context, app *App, ref, what string, n networkAPI, res monitorResolver, mu *sync.Mutex) (spiderw.UnsubscribeFunc, error) {
	switch what {
	case "connected":
		connected, err := n.Connected(ctx)
		if err != nil {
			return nil, err
		}
		if err := printNetworkConnectedLine(app, ref, connected, mu); err != nil {
			return nil, err
		}
		return n.SubscribeConnectedChanged(ctx, func(connected bool) {
			_ = printNetworkConnectedLine(app, ref, connected, mu)
		})

	case "known-network":
		kn, err := n.KnownNetwork(ctx)
		if err != nil {
			return nil, err
		}
		if err := printNetworkKnownNetworkLine(app, ref, res.knownNetworkRef(ctx, kn), mu); err != nil {
			return nil, err
		}
		// Fires when the network is saved (gains a record) or forgotten (loses it).
		// iwd signals a forget by invalidating the property, which the subscription
		// delivers as nil.
		return n.SubscribeKnownNetworkChanged(ctx, func(path *string) {
			_ = printNetworkKnownNetworkLine(app, ref, res.knownNetworkRef(ctx, path), mu)
		})

	case "bsses":
		ess, err := n.ExtendedServiceSet(ctx)
		if err != nil {
			return nil, err
		}
		if err := printNetworkBSSesLine(app, ref, ess, mu); err != nil {
			return nil, err
		}
		return n.SubscribeExtendedServiceSetChanged(ctx, func(paths []string) {
			_ = printNetworkBSSesLine(app, ref, paths, mu)
		})
	}

	return nil, fmt.Errorf("%s", networkMonitorUsage)
}

func monitorNetwork(app *App, networkRef string, args []string) error {
	what, err := parseNetworkMonitorTarget(args)
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

	n, err := networkByRef(ctx, client, networkRef)
	if err != nil {
		return err
	}

	var printMu sync.Mutex
	unsubscribe, err := streamNetworkProperty(ctx, app, networkRef, what, n, monitorResolver{client: client}, &printMu)
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
  <network> connect                Connect to the network. PSK: --passphrase=<s>,
                                   --passphrase-stdin, or the prompt. 802.1x:
                                   --username=<id> + --password=<s>/--password-stdin,
                                   or --private-key-passphrase=<s> for EAP-TLS
  <network> connected              Show whether the network is connected
  <network> type                   Show the network type
  <network> device                 Show the owning device object path
  <network> name                   Show the network SSID
  <network> known-network          Show the known-network object path, if any
  <network> bsses                  List the network's basic service set paths
  <network> monitor connected          Stream the connected flag until Ctrl-C
  <network> monitor known-network      Stream the saved-profile link; fires when
                                       the network is saved or forgotten
  <network> monitor bsses              Stream the network's BSS list`

func networkCommand(app *App) *Command {
	return &Command{
		Name:        "network",
		Description: "Inspect, query, and connect to iwd networks",
		HelpText:    networkHelpText,
		SubUsage:    map[string]string{"monitor": networkMonitorUsage},
		Execute: func(args []string) error {
			return runNetwork(app, args)
		},
	}
}

func printNetworkUsage(app *App) {
	networkCommand(app).printUsage(app)
}
