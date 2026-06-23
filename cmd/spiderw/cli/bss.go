package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/chrispypip/spiderw"
)

type bssRefResult struct {
	Path    string `json:"Path"`
	Address string `json:"Address"`
}

type bssListResult []bssRefResult

// String returns the CLI string form of the value.
func (r bssListResult) String() string {
	if len(r) == 0 {
		return "no basic service sets available"
	}

	var b strings.Builder
	for i, ref := range r {
		if i > 0 {
			b.WriteByte('\n')
		}
		if ref.Address == "" {
			b.WriteString(ref.Path)
			continue
		}
		fmt.Fprintf(&b, "%s\t%s", ref.Address, ref.Path)
	}
	return b.String()
}

func bssRefs(ctx context.Context, client clientAPI) ([]spiderw.BasicServiceSetRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.BasicServiceSets(ctx)
}

func runBSSList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown bss list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := bssRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(bssListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, bssRefResult{Path: ref.Path, Address: ref.Address})
	}
	return app.printOutput(out)
}

type bssStatusEntry struct {
	Path    string `json:"Path"`
	Address string `json:"Address"`
}

type bssStatusResult []bssStatusEntry

// String returns the CLI string form of the value.
func (r bssStatusResult) String() string {
	if len(r) == 0 {
		return "no basic service sets available"
	}

	value := func(v string) string {
		if v == "" {
			return "-"
		}
		return v
	}
	field := func(label, value string) string {
		return fmt.Sprintf("%-12s%s", label+":", value)
	}

	blocks := make([]string, 0, len(r))
	for _, entry := range r {
		lines := []string{
			field("Path", entry.Path),
			field("Address", value(entry.Address)),
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func bssStatusEntryFromBSS(ctx context.Context, b bssAPI) (bssStatusEntry, error) {
	// One Properties.GetAll call per BSS instead of one Get per property.
	props, err := b.Properties(ctx)
	if err != nil {
		return bssStatusEntry{}, err
	}

	return bssStatusEntry{
		Path:    b.Path(),
		Address: props.Address,
	}, nil
}

func runBSSStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown bss status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	bsses, err := client.AllBasicServiceSets(ctx)
	if err != nil {
		return err
	}

	out := make(bssStatusResult, 0, len(bsses))
	for _, b := range bsses {
		entry, err := bssStatusEntryFromBSS(ctx, b)
		if err != nil {
			return err
		}

		out = append(out, entry)
	}
	return app.printOutput(out)
}

func runBSSSingleStatus(app *App, ctx context.Context, bssRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw bss <bss> status")
	}

	return withBSS(app, ctx, bssRef, func(ctx context.Context, b bssAPI) error {
		entry, err := bssStatusEntryFromBSS(ctx, b)
		if err != nil {
			return err
		}

		return app.printOutput(bssStatusResult{entry})
	})
}

func bssByRef(ctx context.Context, client clientAPI, ref string) (bssAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("basic service set reference required")
	}

	refs, err := bssRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no basic service sets available")
	}

	var matches []spiderw.BasicServiceSetRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Address == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("basic service set %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("basic service set reference %q is ambiguous; use a path", ref)
	}

	return client.BasicServiceSet(ctx, matches[0].Path)
}

func withBSS(app *App, ctx context.Context, bssRef string, fn func(context.Context, bssAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		b, err := bssByRef(ctx, client, bssRef)
		if err != nil {
			return err
		}

		return fn(ctx, b)
	})
}

type bssStringResult struct {
	BSS   string `json:"BSS"`
	Value string `json:"Value"`
}

// String returns the CLI string form of the value.
func (r bssStringResult) String() string {
	return r.Value
}

func runBSSAddress(app *App, ctx context.Context, bssRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw bss <bss> address")
	}

	return withBSS(app, ctx, bssRef, func(ctx context.Context, b bssAPI) error {
		value, err := b.Address(ctx)
		if err != nil {
			return err
		}

		return app.printOutput(bssStringResult{BSS: bssRef, Value: value})
	})
}

func runBSSWithRef(app *App, args []string) error {
	if len(args) < 2 {
		printBSSUsage(app)
		return fmt.Errorf("missing bss command for %q", args[0])
	}

	bssRef := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "status":
		return runBSSSingleStatus(app, ctx, bssRef, rest)
	case "address":
		return runBSSAddress(app, ctx, bssRef, rest)
	default:
		printBSSUsage(app)
		return fmt.Errorf("unknown bss command %q for basic service set %q", op, bssRef)
	}
}

func runBSS(app *App, args []string) error {
	if len(args) == 0 {
		printBSSUsage(app)
		return fmt.Errorf("missing bss command")
	}

	switch args[0] {
	case "list":
		return runBSSList(app, args[1:])
	case "status":
		return runBSSStatus(app, args[1:])
	}

	return runBSSWithRef(app, args)
}

const bssHelpText = `Commands:
  list                  List basic service sets (address and path)
  status                Show a snapshot of every basic service set
  <bss> status          Show a snapshot of one basic service set
  <bss> address         Show the basic service set hardware (BSSID) address`

func bssCommand(app *App) *Command {
	return &Command{
		Name:        "bss",
		Description: "Inspect and query iwd basic service sets",
		HelpText:    bssHelpText,
		Execute: func(args []string) error {
			return runBSS(app, args)
		},
	}
}

func printBSSUsage(app *App) {
	bssCommand(app).printUsage(app)
}
