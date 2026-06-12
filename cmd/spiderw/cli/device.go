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

type deviceRefResult struct {
	Path string `json:"Path"`
	Name string `json:"Name"`
}

type deviceListResult []deviceRefResult

// String returns the CLI string form of the value.
func (r deviceListResult) String() string {
	if len(r) == 0 {
		return "no devices available"
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

func deviceRefs(ctx context.Context, client clientAPI) ([]spiderw.DeviceRef, error) {
	if client == nil {
		return nil, fmt.Errorf("client not available")
	}
	daemon := client.Daemon()
	if daemon == nil {
		return nil, fmt.Errorf("daemon not available")
	}
	return daemon.Devices(ctx)
}

func runDeviceList(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown device list argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	refs, err := deviceRefs(ctx, client)
	if err != nil {
		return err
	}

	out := make(deviceListResult, 0, len(refs))
	for _, ref := range refs {
		out = append(out, deviceRefResult{Path: ref.Path, Name: ref.Name})
	}
	return app.printOutput(out)
}

type deviceStatusEntry struct {
	Path    string `json:"Path"`
	Name    string `json:"Name"`
	Address string `json:"Address"`
	Powered bool   `json:"Powered"`
	Mode    string `json:"Mode"`
	Adapter string `json:"Adapter"`
}

type deviceStatusResult []deviceStatusEntry

// String returns the CLI string form of the value.
func (r deviceStatusResult) String() string {
	if len(r) == 0 {
		return "no devices available"
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
		name := entry.Name
		if name == "" {
			name = "(unnamed)"
		}

		lines := []string{
			field("Name", name),
			field("Path", entry.Path),
			field("Address", value(entry.Address)),
			field("Powered", fmt.Sprintf("%t", entry.Powered)),
			field("Mode", value(entry.Mode)),
			field("Adapter", value(entry.Adapter)),
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func deviceStatusEntryFromDevice(ctx context.Context, d deviceAPI) (deviceStatusEntry, error) {
	// One Properties.GetAll call per device instead of one Get per property.
	props, err := d.Properties(ctx)
	if err != nil {
		return deviceStatusEntry{}, err
	}

	return deviceStatusEntry{
		Path:    d.Path(),
		Name:    props.Name,
		Address: props.Address,
		Powered: props.Powered,
		Mode:    props.Mode.String(),
		Adapter: props.Adapter,
	}, nil
}

func runDeviceStatus(app *App, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown device status argument: %s", args[0])
	}
	ctx := context.Background()
	client, err := app.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	devices, err := client.AllDevices(ctx)
	if err != nil {
		return err
	}

	out := make(deviceStatusResult, 0, len(devices))
	for _, d := range devices {
		entry, err := deviceStatusEntryFromDevice(ctx, d)
		if err != nil {
			return err
		}

		out = append(out, entry)
	}
	return app.printOutput(out)
}

func runDeviceSingleStatus(app *App, ctx context.Context, deviceRef string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: spiderw device <device> status")
	}

	return withDevice(app, ctx, deviceRef, func(ctx context.Context, d deviceAPI) error {
		entry, err := deviceStatusEntryFromDevice(ctx, d)
		if err != nil {
			return err
		}

		return app.printOutput(deviceStatusResult{entry})
	})
}

func deviceByRef(ctx context.Context, client clientAPI, ref string) (deviceAPI, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("device reference required")
	}

	refs, err := deviceRefs(ctx, client)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("no devices available")
	}

	var matches []spiderw.DeviceRef
	for _, candidate := range refs {
		if candidate.Path == ref || candidate.Name == ref {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("device %q not found", ref)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("device reference %q is ambiguous; use a device path", ref)
	}

	return client.Device(ctx, matches[0].Path)
}

func withDevice(app *App, ctx context.Context, deviceRef string, fn func(context.Context, deviceAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		d, err := deviceByRef(ctx, client, deviceRef)
		if err != nil {
			return err
		}

		return fn(ctx, d)
	})
}

type devicePoweredResult struct {
	Device  string `json:"Device"`
	Powered bool   `json:"Powered"`
}

// String returns the CLI string form of the value.
func (r devicePoweredResult) String() string {
	return fmt.Sprintf("%t", r.Powered)
}

type deviceModeResult struct {
	Device string `json:"Device"`
	Mode   string `json:"Mode"`
}

// String returns the CLI string form of the value.
func (r deviceModeResult) String() string {
	return r.Mode
}

type deviceStringResult struct {
	Device string `json:"Device"`
	Value  string `json:"Value"`
}

// String returns the CLI string form of the value.
func (r deviceStringResult) String() string {
	return r.Value
}

func runDevicePowered(app *App, ctx context.Context, deviceRef string, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: spiderw device <device> powered [true|false]")
	}

	return withDevice(app, ctx, deviceRef, func(ctx context.Context, d deviceAPI) error {
		if len(args) == 0 {
			powered, err := d.Powered(ctx)
			if err != nil {
				return err
			}
			return app.printOutput(devicePoweredResult{Device: deviceRef, Powered: powered})
		}

		arg := strings.ToLower(strings.TrimSpace(args[0]))
		var powered bool
		switch arg {
		case "true", "1", "yes", "y", "on", "enable", "enabled":
			powered = true
		case "false", "0", "no", "n", "off", "disable", "disabled":
			powered = false
		default:
			return fmt.Errorf("invalid value for device powered: %q (expected true|false)", args[0])
		}

		if err := d.SetPowered(ctx, powered); err != nil {
			return err
		}

		newVal, err := d.Powered(ctx)
		if err != nil {
			return err
		}

		return app.printOutput(devicePoweredResult{Device: deviceRef, Powered: newVal})
	})
}

func runDeviceMode(app *App, ctx context.Context, deviceRef string, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: spiderw device <device> mode [station|ap|ad-hoc]")
	}

	return withDevice(app, ctx, deviceRef, func(ctx context.Context, d deviceAPI) error {
		if len(args) == 0 {
			mode, err := d.Mode(ctx)
			if err != nil {
				return err
			}
			return app.printOutput(deviceModeResult{Device: deviceRef, Mode: mode.String()})
		}

		mode, err := parseModeArg(args[0])
		if err != nil {
			return err
		}

		if err := d.SetMode(ctx, mode); err != nil {
			return err
		}

		newMode, err := d.Mode(ctx)
		if err != nil {
			return err
		}

		return app.printOutput(deviceModeResult{Device: deviceRef, Mode: newMode.String()})
	})
}

func runDeviceString(app *App, ctx context.Context, deviceRef string, args []string, usage string, op func(context.Context, deviceAPI) (string, error)) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}

	return withDevice(app, ctx, deviceRef, func(ctx context.Context, d deviceAPI) error {
		value, err := op(ctx, d)
		if err != nil {
			return err
		}

		return app.printOutput(deviceStringResult{Device: deviceRef, Value: value})
	})
}

func printDevicePoweredLine(app *App, ref string, powered bool, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := devicePoweredResult{Device: ref, Powered: powered}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "powered=%t\n", powered)
	return err
}

func printDeviceModeLine(app *App, ref, mode string, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	out := deviceModeResult{Device: ref, Mode: mode}
	if app != nil && app.Output.JSON {
		return json.NewEncoder(app.stdout()).Encode(out)
	}
	_, err := fmt.Fprintf(app.stdout(), "mode=%s\n", mode)
	return err
}

func monitorDevice(app *App, deviceRef string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: spiderw device <device> monitor <powered|mode>")
	}
	what := args[0]
	if what != "powered" && what != "mode" {
		return fmt.Errorf("usage: spiderw device <device> monitor <powered|mode>")
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

	d, err := deviceByRef(ctx, client, deviceRef)
	if err != nil {
		return err
	}

	var printMu sync.Mutex

	switch what {
	case "powered":
		powered, err := d.Powered(ctx)
		if err != nil {
			return err
		}
		if err := printDevicePoweredLine(app, deviceRef, powered, &printMu); err != nil {
			return err
		}

		unsubscribe, err := d.SubscribePoweredChanged(ctx, func(powered bool) {
			_ = printDevicePoweredLine(app, deviceRef, powered, &printMu)
		})
		if err != nil {
			return err
		}
		defer func() {
			_ = unsubscribe.Unsubscribe()
		}()
	case "mode":
		mode, err := d.Mode(ctx)
		if err != nil {
			return err
		}
		if err := printDeviceModeLine(app, deviceRef, mode.String(), &printMu); err != nil {
			return err
		}

		unsubscribe, err := d.SubscribeModeChanged(ctx, func(mode spiderw.Mode) {
			_ = printDeviceModeLine(app, deviceRef, mode.String(), &printMu)
		})
		if err != nil {
			return err
		}
		defer func() {
			_ = unsubscribe.Unsubscribe()
		}()
	}

	<-ctx.Done()
	return nil
}

func runDeviceWithRef(app *App, args []string) error {
	if len(args) < 2 {
		printDeviceUsage(app)
		return fmt.Errorf("missing device command for %q", args[0])
	}

	deviceRef := args[0]
	op := args[1]
	rest := args[2:]
	ctx := context.Background()

	switch op {
	case "status":
		return runDeviceSingleStatus(app, ctx, deviceRef, rest)
	case "powered":
		return runDevicePowered(app, ctx, deviceRef, rest)
	case "mode":
		return runDeviceMode(app, ctx, deviceRef, rest)
	case "name":
		return runDeviceString(app, ctx, deviceRef, rest, "spiderw device <device> name", func(ctx context.Context, d deviceAPI) (string, error) {
			return d.Name(ctx)
		})
	case "address":
		return runDeviceString(app, ctx, deviceRef, rest, "spiderw device <device> address", func(ctx context.Context, d deviceAPI) (string, error) {
			return d.Address(ctx)
		})
	case "adapter":
		return runDeviceString(app, ctx, deviceRef, rest, "spiderw device <device> adapter", func(ctx context.Context, d deviceAPI) (string, error) {
			return d.Adapter(ctx)
		})
	case "monitor":
		return monitorDevice(app, deviceRef, rest)
	default:
		printDeviceUsage(app)
		return fmt.Errorf("unknown device command %q for device %q", op, deviceRef)
	}
}

func runDevice(app *App, args []string) error {
	if len(args) == 0 {
		printDeviceUsage(app)
		return fmt.Errorf("missing device command")
	}

	switch args[0] {
	case "list":
		return runDeviceList(app, args[1:])
	case "status":
		return runDeviceStatus(app, args[1:])
	}

	return runDeviceWithRef(app, args)
}

const deviceHelpText = `Commands:
  list                                 List devices (name and path)
  status                               Show a snapshot of every device
  <device> status                      Show a snapshot of one device
  <device> powered [true|false]        Get or set the device's powered state
  <device> mode [station|ap|ad-hoc]    Get or set the device's mode
  <device> name                        Show the device name
  <device> address                     Show the device hardware (MAC) address
  <device> adapter                     Show the owning adapter object path
  <device> monitor <powered|mode>      Stream powered or mode changes`

func deviceCommand(app *App) *Command {
	return &Command{
		Name:        "device",
		Description: "Inspect and query iwd devices",
		HelpText:    deviceHelpText,
		Execute: func(args []string) error {
			return runDevice(app, args)
		},
	}
}

func printDeviceUsage(app *App) {
	deviceCommand(app).printUsage(app)
}
