package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/chrispypip/spiderw"
)

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

func withStation(app *App, ctx context.Context, stationRef string, fn func(context.Context, stationAPI) error) error {
	return app.withClient(ctx, func(client clientAPI) error {
		s, err := stationByRef(ctx, client, stationRef)
		if err != nil {
			return err
		}
		return fn(ctx, s)
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
  list                 List stations (object paths)
  status               Show a snapshot of every station
  <station> status     Show a snapshot of one station (by path)

A station is a device in station mode; its read-only connection state covers
State, Scanning, ConnectedNetwork, and the experimental ConnectedAccessPoint and
Affinities. Scanning and disconnect are planned.`

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
