package spiderw_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/chrispypip/spiderw"
)

// ExampleNewClient connects to iwd and reads the daemon version.
func ExampleNewClient() {
	ctx := context.Background()

	// SystemBus is the default and is what real iwd deployments use; pass
	// spiderw.SessionBus to talk to an iwd mock on the session bus instead.
	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	version, err := client.Daemon().Version(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(version)
}

// ExampleDaemon_Info reads the iwd daemon metadata.
func ExampleDaemon_Info() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	info, err := client.Daemon().Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("version=%s stateDir=%s netConfig=%t\n",
		info.Version, info.StateDirectory, info.NetworkConfigurationEnabled)
}

// ExampleDaemon_Adapters lists the adapters iwd currently exposes.
func ExampleDaemon_Adapters() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().Adapters(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, ref := range refs {
		fmt.Printf("%s (%s)\n", ref.Name, ref.Path)
	}
}

// ExampleClient_Adapter discovers an adapter and reads its powered state.
func ExampleClient_Adapter() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().Adapters(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no adapters found")
	}

	adapter, err := client.Adapter(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	powered, err := adapter.Powered(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s powered: %t\n", refs[0].Name, powered)
}

// ExampleClient_AllAdapters constructs a handle for every adapter iwd exposes
// and reports each one's powered state.
func ExampleClient_AllAdapters() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// AllAdapters enumerates via the daemon and returns a live handle per
	// adapter, in enumeration order. Use it when you want to operate on every
	// adapter; use Daemon.Adapters for lightweight references only.
	adapters, err := client.AllAdapters(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, adapter := range adapters {
		name, err := adapter.Name(ctx)
		if err != nil {
			log.Fatal(err)
		}
		powered, err := adapter.Powered(ctx)
		if err != nil {
			log.Fatal(err)
		}
		// Path is static identity and needs no D-Bus call.
		fmt.Printf("%s (%s) powered: %t\n", name, adapter.Path(), powered)
	}
}

// ExampleAdapter_SupportsMode checks whether an adapter supports station mode.
func ExampleAdapter_SupportsMode() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	adapter, err := client.Adapter(ctx, "/net/connman/iwd/0")
	if err != nil {
		log.Fatal(err)
	}

	ok, err := adapter.SupportsMode(ctx, spiderw.AdapterModeStation)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("supports station mode:", ok)
}

// ExampleAdapter_SubscribePoweredChanged registers a callback for powered-state
// changes and unsubscribes when finished.
func ExampleAdapter_SubscribePoweredChanged() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	adapter, err := client.Adapter(ctx, "/net/connman/iwd/0")
	if err != nil {
		log.Fatal(err)
	}

	unsubscribe, err := adapter.SubscribePoweredChanged(ctx, func(powered bool) {
		fmt.Println("powered changed:", powered)
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = unsubscribe.Unsubscribe() }()

	// ... do work while the subscription is active ...
}

// Example_errorHandling shows how to classify a failure with the public error
// sentinels and inspect its structured fields.
func Example_errorHandling() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	_, err = client.Daemon().Info(ctx)
	if err != nil {
		// Match a category with the sentinel.
		if errors.Is(err, spiderw.ErrUnavailable) {
			fmt.Println("iwd is unavailable")
		}

		// Inspect the structured fields with errors.As.
		var swErr *spiderw.Error
		if errors.As(err, &swErr) && swErr.Resource == spiderw.ResourceDaemon {
			fmt.Printf("daemon error in %s: %v\n", swErr.Op, err)
		}
	}
}
