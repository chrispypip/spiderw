package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/chrispypip/spiderw"
)

// accessPointRefs lists the access points (devices currently in AP mode).
func accessPointRefs(ctx context.Context, client clientAPI) ([]spiderw.AccessPointRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.AccessPoints(ctx)
}

type accessPointRefResult struct {
	Path string `json:"Path"`
	Name string `json:"Name"`
}

type accessPointListResult []accessPointRefResult

// String returns the CLI string form of the value.
func (r accessPointListResult) String() string {
	if len(r) == 0 {
		return "no access points"
	}
	var b strings.Builder
	for i, e := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s\t%s", e.Name, e.Path)
	}
	return b.String()
}

func runAccessPointList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown access-point list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := accessPointRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(accessPointListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, accessPointRefResult{Path: ref.Path, Name: ref.Name})
	}
	return app.printOutput(out)
}

type accessPointStatusEntry struct {
	Path            string   `json:"Path"`
	Name            string   `json:"Name"`
	Started         bool     `json:"Started"`
	Scanning        bool     `json:"Scanning"`
	SSID            *string  `json:"SSID,omitempty"`
	Frequency       *uint32  `json:"Frequency,omitempty"`
	PairwiseCiphers []string `json:"PairwiseCiphers,omitempty"`
	GroupCipher     *string  `json:"GroupCipher,omitempty"`
}

type accessPointStatusResult []accessPointStatusEntry

// String returns the CLI string form of the value.
func (r accessPointStatusResult) String() string {
	if len(r) == 0 {
		return "no access points"
	}
	var b strings.Builder
	for i, e := range r {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "%s (%s)\n  Started:  %t", e.Name, e.Path, e.Started)
		// iwd reports Scanning (and the fields below) only while the AP is running,
		// so a stopped AP shows just Started.
		if e.Started {
			fmt.Fprintf(&b, "\n  Scanning: %t", e.Scanning)
		}
		if e.SSID != nil {
			fmt.Fprintf(&b, "\n  SSID:     %s", *e.SSID)
		}
		if e.Frequency != nil {
			fmt.Fprintf(&b, "\n  Frequency: %d MHz", *e.Frequency)
		}
		if len(e.PairwiseCiphers) > 0 {
			fmt.Fprintf(&b, "\n  Pairwise: %s", strings.Join(e.PairwiseCiphers, ", "))
		}
		if e.GroupCipher != nil {
			fmt.Fprintf(&b, "\n  Group:    %s", *e.GroupCipher)
		}
	}
	return b.String()
}

func accessPointStatusEntryFrom(ctx context.Context, a accessPointAPI) (accessPointStatusEntry, error) {
	props, err := a.Properties(ctx)
	if err != nil {
		return accessPointStatusEntry{}, err
	}
	return accessPointStatusEntry{
		Path:            a.Path(),
		Name:            a.Name(),
		Started:         props.Started,
		Scanning:        props.Scanning,
		SSID:            props.SSID,
		Frequency:       props.Frequency,
		PairwiseCiphers: props.PairwiseCiphers,
		GroupCipher:     props.GroupCipher,
	}, nil
}

func runAccessPointStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown access-point status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	aps, err := client.AllAccessPoints(ctx)
	if err != nil {
		return err
	}

	out := make(accessPointStatusResult, 0, len(aps))
	for _, a := range aps {
		entry, err := accessPointStatusEntryFrom(ctx, a)
		if err != nil {
			return err
		}
		out = append(out, entry)
	}
	return app.printOutput(out)
}

func runAccessPointSingleStatus(app *App, ctx context.Context, apRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw access-point <ap> status")
	}
	return withAccessPoint(app, ctx, apRef, func(ctx context.Context, a accessPointAPI) error {
		entry, err := accessPointStatusEntryFrom(ctx, a)
		if err != nil {
			return err
		}
		return app.printOutput(accessPointStatusResult{entry})
	})
}

func accessPointByRef(ctx context.Context, client clientAPI, ref string) (accessPointAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("access point reference required")
	}

	refs, err := accessPointRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no access points available")
	}

	var matches []spiderw.AccessPointRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Name == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("access point %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("access point reference %q is ambiguous; use an object path", ref)
	}
	return client.AccessPoint(ctx, matches[0].Path)
}

func withAccessPoint(app *App, ctx context.Context, apRef string, fn func(ctx context.Context, a accessPointAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		a, err := accessPointByRef(ctx, client, apRef)
		if err != nil {
			return err
		}
		return fn(ctx, a)
	})
}

// accessPointScanStartedResult is printed in `scan` wait mode once the scan has
// been triggered, before blocking for it to finish.
type accessPointScanStartedResult struct {
	AccessPoint string `json:"AccessPoint"`
}

// String returns the CLI string form of the value.
func (r accessPointScanStartedResult) String() string {
	return fmt.Sprintf("access point %q: scan started; waiting for it to finish...", r.AccessPoint)
}

type accessPointResult struct {
	AccessPoint string `json:"AccessPoint"`
	Action      string `json:"Action"`
}

// String returns the CLI string form of the value.
func (r accessPointResult) String() string {
	switch r.Action {
	case "stop":
		return "access point stopped"
	case "scan":
		return "scan started"
	default:
		return "access point started"
	}
}

func runAccessPointStart(app *App, ctx context.Context, apRef string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: spiderw access-point <ap> start <ssid> <passphrase>")
	}
	return withAccessPoint(app, ctx, apRef, func(ctx context.Context, a accessPointAPI) error {
		if err := a.Start(ctx, args[0], args[1]); err != nil {
			return err
		}
		return app.printOutput(accessPointResult{AccessPoint: apRef, Action: "start"})
	})
}

func runAccessPointStartProfile(app *App, ctx context.Context, apRef string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: spiderw access-point <ap> start-profile <ssid>")
	}
	return withAccessPoint(app, ctx, apRef, func(ctx context.Context, a accessPointAPI) error {
		if err := a.StartProfile(ctx, args[0]); err != nil {
			return err
		}
		return app.printOutput(accessPointResult{AccessPoint: apRef, Action: "start-profile"})
	})
}

func runAccessPointStop(app *App, ctx context.Context, apRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw access-point <ap> stop")
	}
	return withAccessPoint(app, ctx, apRef, func(ctx context.Context, a accessPointAPI) error {
		if err := a.Stop(ctx); err != nil {
			return err
		}
		return app.printOutput(accessPointResult{AccessPoint: apRef, Action: "stop"})
	})
}

func runAccessPointScan(app *App, ctx context.Context, apRef string, args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(app.stderr())
	noWait := fs.Bool("no-wait", false, "trigger the scan and return without waiting for it to finish")
	timeout := fs.Duration("timeout", scanWaitTimeout, "how long to wait for the scan to finish (wait mode)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: spiderw access-point <ap> scan [--no-wait] [--timeout=<duration>]")
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

	return withAccessPoint(app, ctx, apRef, func(ctx context.Context, a accessPointAPI) error {
		if *noWait {
			if err := a.Scan(ctx); err != nil {
				return err
			}
			return app.printOutput(accessPointResult{AccessPoint: apRef, Action: "scan"})
		}

		// Wait: subscribe to Scanning, start the scan, then block until Scanning
		// returns to false. Subscribing before Scan avoids missing the transition.
		done := make(chan struct{}, 1)
		unsubscribe, err := a.SubscribeScanningChanged(ctx, func(scanning bool) {
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

		if err := a.Scan(ctx); err != nil {
			return err
		}

		// Signal that the scan started before blocking on the result, mirroring the
		// wsc commands (which print before their blocking enrollment).
		if err := app.printOutput(accessPointScanStartedResult{AccessPoint: apRef}); err != nil {
			return err
		}

		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(*timeout):
			return fmt.Errorf("timed out waiting for scan to finish after %s", *timeout)
		}

		return printAccessPointNetworks(app, ctx, a)
	})
}

type accessPointNetworkResult struct {
	Name      string  `json:"Name"`
	SignalDBm float64 `json:"SignalDBm"`
	Type      string  `json:"Type"`
}

type accessPointNetworksResult []accessPointNetworkResult

// String returns the CLI string form of the value.
func (r accessPointNetworksResult) String() string {
	if len(r) == 0 {
		return "no networks available"
	}
	var b strings.Builder
	for i, n := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s\t%g dBm\t%s", n.Name, n.SignalDBm, n.Type)
	}
	return b.String()
}

// printAccessPointNetworks reads the AP's last scan results and renders them; it
// is shared by the `networks` subcommand and `scan` (wait mode).
func printAccessPointNetworks(app *App, ctx context.Context, a accessPointAPI) error {
	nets, err := a.OrderedNetworks(ctx)
	if err != nil {
		return err
	}
	out := make(accessPointNetworksResult, 0, len(nets))
	for _, n := range nets {
		out = append(out, accessPointNetworkResult{Name: n.Name, SignalDBm: n.SignalStrength, Type: n.Type.String()})
	}
	return app.printOutput(out)
}

func runAccessPointNetworks(app *App, ctx context.Context, apRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw access-point <ap> networks")
	}
	return withAccessPoint(app, ctx, apRef, func(ctx context.Context, a accessPointAPI) error {
		return printAccessPointNetworks(app, ctx, a)
	})
}

func runAccessPointWithRef(app *App, args []string) error {
	if len(args) < 2 {
		printAccessPointUsage(app)
		return fmt.Errorf("missing access-point command for %q", args[0])
	}

	apRef := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "status":
		return runAccessPointSingleStatus(app, ctx, apRef, rest)
	case "start":
		return runAccessPointStart(app, ctx, apRef, rest)
	case "start-profile":
		return runAccessPointStartProfile(app, ctx, apRef, rest)
	case "stop":
		return runAccessPointStop(app, ctx, apRef, rest)
	case "scan":
		return runAccessPointScan(app, ctx, apRef, rest)
	case "networks":
		return runAccessPointNetworks(app, ctx, apRef, rest)
	default:
		printAccessPointUsage(app)
		return fmt.Errorf("unknown access-point command %q for access point %q", op, apRef)
	}
}

func runAccessPoint(app *App, args []string) error {
	if len(args) == 0 {
		printAccessPointUsage(app)
		return fmt.Errorf("missing access-point command")
	}

	switch args[0] {
	case "list":
		return runAccessPointList(app, args[1:])
	case "status":
		return runAccessPointStatus(app, args[1:])
	}

	return runAccessPointWithRef(app, args)
}

const accessPointHelpText = `Commands:
  list                                  List access points (device in AP mode)
  status                                Show a snapshot of every access point
  <ap> status                           Show a snapshot of one access point
  <ap> start <ssid> <passphrase>        Start a PSK access point
  <ap> start-profile <ssid>             Start an access point from a stored profile
  <ap> stop                             Stop the access point
  <ap> scan [--no-wait] [--timeout=<dur>]  Scan for nearby networks (waits for the
                                        scan to finish, then lists results, unless
                                        --no-wait; --timeout bounds the wait,
                                        default 15s)
  <ap> networks                         List networks from the last scan

An access point is a device in AP mode (Device.SetMode "ap"); it is referenced by
its device name (e.g. "wlan1") or object path.`

func accessPointCommand(app *App) *Command {
	return &Command{
		Name:        "access-point",
		Description: "Inspect and control iwd access points (AP-mode devices)",
		HelpText:    accessPointHelpText,
		Execute: func(args []string) error {
			return runAccessPoint(app, args)
		},
	}
}

func printAccessPointUsage(app *App) {
	accessPointCommand(app).printUsage(app)
}
