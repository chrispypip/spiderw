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

type adapterListResult []adapterRefResult

type adapterRefResult struct {
	Path string `json:"Path"`
	Name string `json:"Name"`
}

// String returns the CLI string form of the value.
func (r adapterListResult) String() string {
	if len(r) == 0 {
		return "no adapters available"
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

func adapterRefs(ctx context.Context, client clientAPI) ([]spiderw.AdapterRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.Adapters(ctx)
}

func adapterByRef(ctx context.Context, client clientAPI, ref string) (adapterAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("adapter reference required")
	}

	refs, err := adapterRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no adapters available")
	}

	var matches []spiderw.AdapterRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Name == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("adapter %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("adapter reference %q is ambiguous; use an adapter path", ref)
	}

	return client.Adapter(ctx, matches[0].Path)
}

type adapterPoweredResult struct {
	Adapter string `json:"Adapter"`
	Powered bool   `json:"Powered"`
}

// String returns the CLI string form of the value.
func (r adapterPoweredResult) String() string {
	return fmt.Sprintf("%t", r.Powered)
}

type adapterBoolResult struct {
	Adapter string `json:"Adapter"`
	Value   bool   `json:"Value"`
}

// String returns the CLI string form of the value.
func (r adapterBoolResult) String() string {
	return fmt.Sprintf("%t", r.Value)
}

type adapterStringResult struct {
	Adapter string `json:"Adapter"`
	Value   string `json:"Value"`
}

// String returns the CLI string form of the value.
func (r adapterStringResult) String() string {
	return r.Value
}

type adapterOptionalStringResult struct {
	Adapter string  `json:"Adapter"`
	Value   *string `json:"Value"`
}

// String returns the CLI string form of the value.
func (r adapterOptionalStringResult) String() string {
	if r.Value == nil {
		return ""
	}
	return *r.Value
}

type adapterSupportedModesResult struct {
	Adapter        string   `json:"Adapter"`
	SupportedModes []string `json:"SupportedModes"`
}

// String returns the CLI string form of the value.
func (r adapterSupportedModesResult) String() string {
	return strings.Join(r.SupportedModes, "\n")
}

func printAdapterPoweredLine(app *App, ref string, powered bool, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := adapterPoweredResult{Adapter: ref, Powered: powered}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "powered=%t\n", powered)
	return err
}

func parseModeArg(raw string) (spiderw.Mode, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "station", "sta":
		return spiderw.ModeStation, nil
	case "ap", "access-point", "accesspoint":
		return spiderw.ModeAP, nil
	case "ad-hoc", "adhoc", "ad_hoc", "ibss":
		return spiderw.ModeAdHoc, nil
	default:
		return spiderw.ModeUnknown, fmt.Errorf("invalid mode %q (expected station, ap, or ad-hoc)", raw)
	}
}

func withAdapter(app *App, ctx context.Context, adapterRef string, fn func(context.Context, adapterAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		a, err := adapterByRef(ctx, client, adapterRef)
		if err != nil {
			return err
		}

		return fn(ctx, a)
	})
}

func getAdapterBool(app *App, ctx context.Context, adapterRef string, usage string, args []string, op func(context.Context, adapterAPI) (bool, error)) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}

	return withAdapter(app, ctx, adapterRef, func(ctx context.Context, a adapterAPI) error {
		value, err := op(ctx, a)
		if err != nil {
			return err
		}

		return app.printOutput(adapterBoolResult{Adapter: adapterRef, Value: value})
	})
}

func monitorAdapterPowered(app *App, adapterRef string, args []string) error {
	if len(args) != 1 || args[0] != "powered" {
		return fmt.Errorf("usage: spiderw adapter <adapter> monitor powered")
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

	a, err := adapterByRef(ctx, client, adapterRef)
	if err != nil {
		return err
	}

	var printMu sync.Mutex

	powered, err := a.Powered(ctx)
	if err != nil {
		return err
	}
	if err := printAdapterPoweredLine(app, adapterRef, powered, &printMu); err != nil {
		return err
	}

	unsubscribe, err := a.SubscribePoweredChanged(ctx, func(powered bool) {
		_ = printAdapterPoweredLine(app, adapterRef, powered, &printMu)
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

func runAdapterList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown adapter list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := adapterRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(adapterListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, adapterRefResult{Path: ref.Path, Name: ref.Name})
	}
	return app.printOutput(out)
}

type adapterStatusEntry struct {
	Path           string   `json:"Path"`
	Name           string   `json:"Name"`
	Powered        bool     `json:"Powered"`
	Model          *string  `json:"Model"`
	Vendor         *string  `json:"Vendor"`
	SupportedModes []string `json:"SupportedModes"`
}

type adapterStatusResult []adapterStatusEntry

// String returns the CLI string form of the value.
func (r adapterStatusResult) String() string {
	if len(r) == 0 {
		return "no adapters available"
	}

	optional := func(v *string) string {
		if v == nil || *v == "" {
			return "-"
		}
		return *v
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
		modes := "-"
		if len(entry.SupportedModes) > 0 {
			modes = strings.Join(entry.SupportedModes, ", ")
		}

		lines := []string{
			field("Name", name),
			field("Path", entry.Path),
			field("Powered", fmt.Sprintf("%t", entry.Powered)),
			field("Model", optional(entry.Model)),
			field("Vendor", optional(entry.Vendor)),
			field("SupportedModes", modes),
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func runAdapterStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown adapter status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	adapters, err := client.AllAdapters(ctx)
	if err != nil {
		return err
	}

	out := make(adapterStatusResult, 0, len(adapters))
	for _, a := range adapters {
		// One Properties.GetAll call per adapter instead of one Get per
		// property. An absent optional (Model/Vendor) is simply missing from
		// the reply and stays nil; any error is a real failure and surfaces.
		props, err := a.Properties(ctx)
		if err != nil {
			return err
		}

		modeStrs := make([]string, 0, len(props.SupportedModes))
		for _, mode := range props.SupportedModes {
			modeStrs = append(modeStrs, mode.String())
		}

		out = append(out, adapterStatusEntry{
			Path:           a.Path(),
			Name:           props.Name,
			Powered:        props.Powered,
			Model:          props.Model,
			Vendor:         props.Vendor,
			SupportedModes: modeStrs,
		})
	}
	return app.printOutput(out)
}

func runAdapterPowered(app *App, ctx context.Context, adapterRef string, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: spiderw adapter <adapter> powered [true|false]")
	}

	return withAdapter(app, ctx, adapterRef, func(ctx context.Context, a adapterAPI) error {
		if len(args) == 0 {
			powered, err := a.Powered(ctx)
			if err != nil {
				return err
			}
			return app.printOutput(adapterPoweredResult{Adapter: adapterRef, Powered: powered})
		}

		arg := strings.ToLower(strings.TrimSpace(args[0]))
		var powered bool
		switch arg {
		case "true", "1", "yes", "y", "on", "enable", "enabled":
			powered = true
		case "false", "0", "no", "n", "off", "disable", "disabled":
			powered = false
		default:
			return fmt.Errorf("invalid value for adapter powered: %q (expected true|false)", args[0])
		}

		if err := a.SetPowered(ctx, powered); err != nil {
			return err
		}

		newVal, err := a.Powered(ctx)
		if err != nil {
			return err
		}

		return app.printOutput(adapterPoweredResult{Adapter: adapterRef, Powered: newVal})
	})
}

func runAdapterString(app *App, ctx context.Context, adapterRef string, args []string, usage string, op func(context.Context, adapterAPI) (string, error)) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}

	return withAdapter(app, ctx, adapterRef, func(ctx context.Context, a adapterAPI) error {
		value, err := op(ctx, a)
		if err != nil {
			return err
		}

		return app.printOutput(adapterStringResult{Adapter: adapterRef, Value: value})
	})
}

func runAdapterOptionalString(app *App, ctx context.Context, adapterRef string, args []string, usage string, op func(context.Context, adapterAPI) (*string, error)) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}

	return withAdapter(app, ctx, adapterRef, func(ctx context.Context, a adapterAPI) error {
		value, err := op(ctx, a)
		if err != nil {
			return err
		}

		return app.printOutput(adapterOptionalStringResult{Adapter: adapterRef, Value: value})
	})
}

func runAdapterSupportedModes(app *App, ctx context.Context, adapterRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw adapter <adapter> supported-modes")
	}

	return withAdapter(app, ctx, adapterRef, func(ctx context.Context, a adapterAPI) error {
		modes, err := a.SupportedModes(ctx)
		if err != nil {
			return err
		}

		out := make([]string, 0, len(modes))
		for _, mode := range modes {
			out = append(out, mode.String())
		}

		return app.printOutput(adapterSupportedModesResult{Adapter: adapterRef, SupportedModes: out})
	})
}

func runAdapterSupportsMode(app *App, ctx context.Context, adapterRef string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: spiderw adapter <adapter> supports-mode <station|ap|ad-hoc>")
	}

	mode, err := parseModeArg(args[0])
	if err != nil {
		return err
	}

	return withAdapter(app, ctx, adapterRef, func(ctx context.Context, a adapterAPI) error {
		value, err := a.SupportsMode(ctx, mode)
		if err != nil {
			return err
		}

		return app.printOutput(adapterBoolResult{Adapter: adapterRef, Value: value})
	})
}

func runAdapterWithRef(app *App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: spiderw adapter <adapter> <command>")
	}

	adapterRef := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "powered":
		return runAdapterPowered(app, ctx, adapterRef, rest)
	case "name":
		return runAdapterString(app, ctx, adapterRef, rest, "spiderw adapter <adapter> name", func(ctx context.Context, a adapterAPI) (string, error) {
			return a.Name(ctx)
		})
	case "model":
		return runAdapterOptionalString(app, ctx, adapterRef, rest, "spiderw adapter <adapter> model", func(ctx context.Context, a adapterAPI) (*string, error) {
			return a.Model(ctx)
		})
	case "vendor":
		return runAdapterOptionalString(app, ctx, adapterRef, rest, "spiderw adapter <adapter> vendor", func(ctx context.Context, a adapterAPI) (*string, error) {
			return a.Vendor(ctx)
		})
	case "supported-modes":
		return runAdapterSupportedModes(app, ctx, adapterRef, rest)
	case "supports-mode":
		return runAdapterSupportsMode(app, ctx, adapterRef, rest)
	case "supports-station":
		return getAdapterBool(app, ctx, adapterRef, "spiderw adapter <adapter> supports-station", rest, func(ctx context.Context, a adapterAPI) (bool, error) {
			return a.SupportsStation(ctx)
		})
	case "supports-ap":
		return getAdapterBool(app, ctx, adapterRef, "spiderw adapter <adapter> supports-ap", rest, func(ctx context.Context, a adapterAPI) (bool, error) {
			return a.SupportsAP(ctx)
		})
	case "supports-adhoc", "supports-ad-hoc":
		return getAdapterBool(app, ctx, adapterRef, "spiderw adapter <adapter> supports-ad-hoc", rest, func(ctx context.Context, a adapterAPI) (bool, error) {
			return a.SupportsAdHoc(ctx)
		})
	case "monitor":
		return monitorAdapterPowered(app, adapterRef, rest)
	default:
		return fmt.Errorf("unknown adapter command %q for adapter %q", op, adapterRef)
	}
}

func runAdapter(app *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: spiderw adapter list OR spiderw adapter status OR spiderw adapter <adapter> <command>")
	}

	switch args[0] {
	case "list":
		return runAdapterList(app, args[1:])
	case "status":
		return runAdapterStatus(app, args[1:])
	}

	return runAdapterWithRef(app, args)
}

func adapterCommand(app *App) *Command {
	return &Command{
		Name:        "adapter",
		Description: "Inspect and query iwd adapters",
		Execute: func(args []string) error {
			return runAdapter(app, args)
		},
	}
}
