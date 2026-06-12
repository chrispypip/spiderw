package cli

import (
	"context"
	"io"
	"os"

	"github.com/chrispypip/spiderw"
)

type clientFactory func(context.Context, spiderw.Bus) (clientAPI, error)

// App holds per-invocation CLI state.
type App struct {
	// Session selects the session D-Bus bus instead of the default system bus.
	Session bool

	// Help records whether global help was requested.
	Help bool

	// Output controls command output formatting.
	Output outputConfig

	// Stdout receives normal command output.
	Stdout io.Writer

	// Stderr receives diagnostic command output.
	Stderr io.Writer

	// NewClient constructs the spiderw client used by commands.
	NewClient clientFactory
}

func newApp() *App {
	return &App{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		NewClient: defaultClientFactory,
	}
}

func (a *App) newClient(ctx context.Context) (clientAPI, error) {
	if a == nil || a.NewClient == nil {
		return defaultClientFactory(ctx, spiderw.SystemBus)
	}
	bus := spiderw.SystemBus
	if a.Session {
		bus = spiderw.SessionBus
	}
	return a.NewClient(ctx, bus)
}

func (a *App) withClient(ctx context.Context, fn func(clientAPI) error) error {
	client, err := a.newClient(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Close()
	}()

	return fn(client)
}
