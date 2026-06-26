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

// ExampleClient_Device discovers a device and reads its current status.
func ExampleClient_Device() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().Devices(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no devices found")
	}

	device, err := client.Device(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	props, err := device.Properties(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s (%s) mode=%s powered=%t adapter=%s\n",
		props.Name, props.Address, props.Mode, props.Powered, props.Adapter)
}

// ExampleClient_BasicServiceSet discovers a basic service set (BSS) and reads
// its address. A BSS is a single access point/radio the device can see; iwd
// reports one per detected AP.
func ExampleClient_BasicServiceSet() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().BasicServiceSets(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no basic service sets found")
	}

	bss, err := client.BasicServiceSet(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	address, err := bss.Address(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(address)
}

// ExampleClient_AllBasicServiceSets constructs a handle for every basic service
// set iwd exposes and prints each one's address. A device typically sees many
// BSSes — one per access point/radio heard during a scan.
func ExampleClient_AllBasicServiceSets() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// AllBasicServiceSets enumerates via the daemon and returns a live handle per
	// BSS, in enumeration order. Use Daemon.BasicServiceSets for lightweight
	// references only.
	bsses, err := client.AllBasicServiceSets(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, bss := range bsses {
		address, err := bss.Address(ctx)
		if err != nil {
			log.Fatal(err)
		}
		// Path is static identity and needs no D-Bus call.
		fmt.Printf("%s (%s)\n", address, bss.Path())
	}
}

// ExampleClient_Network discovers a network and reads its properties.
func ExampleClient_Network() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().Networks(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no networks found")
	}

	network, err := client.Network(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	props, err := network.Properties(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s type=%s connected=%t\n", props.Name, props.Type, props.Connected)
}

// ExampleNetwork_Connect connects to a network. Open and already-known networks
// connect without a credentials agent; a not-yet-known secured network fails
// with an error matching spiderw.ErrNoAgent.
func ExampleNetwork_Connect() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().Networks(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no networks found")
	}

	network, err := client.Network(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	switch err := network.Connect(ctx); {
	case err == nil:
		fmt.Println("connected")
	case errors.Is(err, spiderw.ErrNoAgent):
		// Connecting to this secured network needs a credentials agent.
		fmt.Println("a credentials agent is required to connect to this network")
	default:
		log.Fatal(err)
	}
}

// ExampleNetwork_ExtendedServiceSet lists the basic service sets (access points)
// that make up a network and resolves each path to a live BasicServiceSet handle.
func ExampleNetwork_ExtendedServiceSet() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().Networks(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no networks found")
	}

	network, err := client.Network(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	// ExtendedServiceSet returns BSS object paths; resolve each with
	// Client.BasicServiceSet. A single network may be served by several BSSes
	// (for example a 2.4 GHz and a 5 GHz radio).
	paths, err := network.ExtendedServiceSet(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, path := range paths {
		bss, err := client.BasicServiceSet(ctx, path)
		if err != nil {
			log.Fatal(err)
		}
		address, err := bss.Address(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(address)
	}
}

// ExampleClient_KnownNetwork discovers a saved (known) network and reads its
// properties.
func ExampleClient_KnownNetwork() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().KnownNetworks(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no known networks found")
	}

	known, err := client.KnownNetwork(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	props, err := known.Properties(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s type=%s autoConnect=%t\n", props.Name, props.Type, props.AutoConnect)
}

// ExampleKnownNetwork_SetAutoConnect disables automatic connection for a saved
// network without forgetting it.
func ExampleKnownNetwork_SetAutoConnect() {
	ctx := context.Background()

	client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	refs, err := client.Daemon().KnownNetworks(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if len(refs) == 0 {
		log.Fatal("no known networks found")
	}

	known, err := client.KnownNetwork(ctx, refs[0].Path)
	if err != nil {
		log.Fatal(err)
	}

	if err := known.SetAutoConnect(ctx, false); err != nil {
		log.Fatal(err)
	}
	// Use known.Forget(ctx) to remove the saved network entirely.
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

// ExampleAdapter_Properties reads every adapter property in a single
// Properties.GetAll call instead of one D-Bus call per property.
func ExampleAdapter_Properties() {
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

	// One round-trip fetches Powered, Name, Model, Vendor, and SupportedModes
	// together. Model and Vendor are nil when iwd does not report them.
	props, err := adapter.Properties(ctx)
	if err != nil {
		log.Fatal(err)
	}

	model := "unknown"
	if props.Model != nil {
		model = *props.Model
	}
	fmt.Printf("%s powered=%t model=%s modes=%v\n",
		props.Name, props.Powered, model, props.SupportedModes)
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

	ok, err := adapter.SupportsMode(ctx, spiderw.ModeStation)
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

		// Inspect the structured fields with errors.AsType.
		if swErr, ok := errors.AsType[*spiderw.Error](err); ok && swErr.Resource == spiderw.ResourceDaemon {
			fmt.Printf("daemon error in %s: %v\n", swErr.Op, err)
		}

	}
}
