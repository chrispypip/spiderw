package cli

import (
	"context"
	"io"
	"os"

	"github.com/chrispypip/spiderw"
)

type clientFactory func(ctx context.Context, bus spiderw.Bus) (clientAPI, error)

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

	// Stdin is the input source for --passphrase-stdin.
	Stdin io.Reader

	// NewClient constructs the spiderw client used by commands.
	NewClient clientFactory

	// PromptPassphrase reads a passphrase interactively. Overridable in tests so
	// the connect flow can be driven without a terminal.
	PromptPassphrase func(prompt string) (string, error)
}

func newApp() *App {
	return &App{
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Stdin:            os.Stdin,
		NewClient:        defaultClientFactory,
		PromptPassphrase: defaultPromptPassphrase,
	}
}

func (a *App) stdin() io.Reader {
	if a == nil || a.Stdin == nil {
		return os.Stdin
	}
	return a.Stdin
}

func (a *App) promptPassphrase(prompt string) (string, error) {
	if a == nil || a.PromptPassphrase == nil {
		return defaultPromptPassphrase(prompt)
	}
	return a.PromptPassphrase(prompt)
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
