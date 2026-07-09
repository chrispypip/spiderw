// Command status prints a read-only overview of the local iwd state: daemon
// metadata plus every adapter, device, station, and known network. It changes
// nothing, so it is a safe first program to run.
//
// It targets the system bus (real iwd) by default; pass -session to run it
// against the iwd mock instead. See examples/README.md.
//
//	go run ./examples/status            # real iwd (system bus)
//	go run ./examples/status -session   # iwd mock (session bus)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/chrispypip/spiderw"
)

func main() {
	session := flag.Bool("session", false, "use the session bus (iwd mock) instead of the system bus")
	flag.Parse()

	ctx := context.Background()

	// SystemBus is the default and is what real iwd deployments use; SessionBus
	// points at an iwd mock registered on the session bus.
	bus := spiderw.SystemBus
	if *session {
		bus = spiderw.SessionBus
	}

	client, err := spiderw.NewClient(ctx, bus)
	if err != nil {
		log.Fatalf("connect to iwd: %v", err)
	}
	defer func() { _ = client.Close() }()

	info, err := client.Daemon().Info(ctx)
	if err != nil {
		log.Fatalf("read daemon info: %v", err)
	}
	fmt.Printf("iwd %s   state-dir=%s   net-config=%t\n",
		info.Version, info.StateDirectory, info.NetworkConfigurationEnabled)

	printAdapters(ctx, client)
	printDevices(ctx, client)
	printStations(ctx, client)
	printKnownNetworks(ctx, client)
}

func printAdapters(ctx context.Context, client *spiderw.Client) {
	adapters, err := client.AllAdapters(ctx)
	if err != nil {
		log.Fatalf("list adapters: %v", err)
	}
	fmt.Printf("\nAdapters (%d):\n", len(adapters))
	for _, a := range adapters {
		// Properties reads every field in a single D-Bus call.
		p, err := a.Properties(ctx)
		if err != nil {
			fmt.Printf("  %-10s <unavailable: %v>\n", a.Path(), err)
			continue
		}
		fmt.Printf("  %-10s powered=%-5t modes=%v\n", p.Name, p.Powered, p.SupportedModes)
	}
}

func printDevices(ctx context.Context, client *spiderw.Client) {
	devices, err := client.AllDevices(ctx)
	if err != nil {
		log.Fatalf("list devices: %v", err)
	}
	fmt.Printf("\nDevices (%d):\n", len(devices))
	for _, d := range devices {
		p, err := d.Properties(ctx)
		if err != nil {
			fmt.Printf("  %-10s <unavailable: %v>\n", d.Path(), err)
			continue
		}
		fmt.Printf("  %-10s mode=%-8s powered=%-5t address=%s\n", p.Name, p.Mode, p.Powered, p.Address)
	}
}

func printStations(ctx context.Context, client *spiderw.Client) {
	stations, err := client.AllStations(ctx)
	if err != nil {
		log.Fatalf("list stations: %v", err)
	}
	fmt.Printf("\nStations (%d):\n", len(stations))
	for _, s := range stations {
		p, err := s.Properties(ctx)
		if err != nil {
			fmt.Printf("  %-10s <unavailable: %v>\n", s.Path(), err)
			continue
		}
		connected := "-"
		if p.ConnectedNetwork != nil {
			connected = p.ConnectedNetwork.Name
		}
		fmt.Printf("  %-10s state=%-13s scanning=%-5t connected=%s\n", s.Name(), p.State, p.Scanning, connected)
	}
}

func printKnownNetworks(ctx context.Context, client *spiderw.Client) {
	known, err := client.AllKnownNetworks(ctx)
	if err != nil {
		log.Fatalf("list known networks: %v", err)
	}
	fmt.Printf("\nKnown networks (%d):\n", len(known))
	for _, k := range known {
		p, err := k.Properties(ctx)
		if err != nil {
			fmt.Printf("  <unavailable: %v>\n", err)
			continue
		}
		fmt.Printf("  %-14s type=%-8s hidden=%-5t autoconnect=%t\n", p.Name, p.Type, p.Hidden, p.AutoConnect)
	}
}
