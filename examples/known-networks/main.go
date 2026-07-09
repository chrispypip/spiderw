// Command known-networks lists the networks iwd has stored credentials for and
// can optionally manage one: toggle its autoconnect flag or forget it entirely.
//
// With no action flags it only lists (read-only). The -forget and -autoconnect
// actions require -name and change stored configuration on a real system.
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/known-networks -session
//	go run ./examples/known-networks -session -name KnownNetwork -autoconnect off
//	go run ./examples/known-networks -session -name KnownNetwork -forget
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
	name := flag.String("name", "", "known network to act on (with -forget or -autoconnect)")
	forget := flag.Bool("forget", false, "forget the -name network")
	autoconnect := flag.String("autoconnect", "", "set autoconnect for the -name network: on|off")
	flag.Parse()

	ctx := context.Background()

	bus := spiderw.SystemBus
	if *session {
		bus = spiderw.SessionBus
	}

	client, err := spiderw.NewClient(ctx, bus)
	if err != nil {
		log.Fatalf("connect to iwd: %v", err)
	}
	defer func() { _ = client.Close() }()

	switch {
	case *name != "" && *forget:
		if err := doForget(ctx, client, *name); err != nil {
			log.Fatalf("forget %q: %v", *name, err)
		}
		fmt.Printf("forgot %q\n", *name)
	case *name != "" && *autoconnect != "":
		on := *autoconnect == "on"
		if err := doSetAutoConnect(ctx, client, *name, on); err != nil {
			log.Fatalf("set autoconnect for %q: %v", *name, err)
		}
		fmt.Printf("set autoconnect=%t for %q\n", on, *name)
	default:
		if err := list(ctx, client); err != nil {
			log.Fatalf("list known networks: %v", err)
		}
	}
}

func list(ctx context.Context, client *spiderw.Client) error {
	known, err := client.AllKnownNetworks(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("known networks (%d):\n", len(known))
	for _, k := range known {
		p, err := k.Properties(ctx)
		if err != nil {
			fmt.Printf("  <unavailable: %v>\n", err)
			continue
		}
		last := "never"
		if p.LastConnectedTime != nil {
			last = *p.LastConnectedTime
		}
		fmt.Printf("  %-14s type=%-8s hidden=%-5t autoconnect=%-5t last=%s\n",
			p.Name, p.Type, p.Hidden, p.AutoConnect, last)
	}
	return nil
}

func doForget(ctx context.Context, client *spiderw.Client, name string) error {
	k, err := find(ctx, client, name)
	if err != nil {
		return err
	}
	return k.Forget(ctx)
}

func doSetAutoConnect(ctx context.Context, client *spiderw.Client, name string, on bool) error {
	k, err := find(ctx, client, name)
	if err != nil {
		return err
	}
	return k.SetAutoConnect(ctx, on)
}

func find(ctx context.Context, client *spiderw.Client, name string) (*spiderw.KnownNetwork, error) {
	known, err := client.AllKnownNetworks(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range known {
		n, err := k.Name(ctx)
		if err != nil {
			continue
		}
		if n == name {
			return k, nil
		}
	}
	return nil, fmt.Errorf("no known network named %q", name)
}
