// Command connect-hidden joins a hidden network — one that does not broadcast
// its SSID and so never appears in scan results. You name it explicitly and iwd
// probes for it. A credentials agent supplies the passphrase.
//
// This changes state on a real system, so both -ssid and -passphrase are
// required (there is no safe default).
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/connect-hidden -session -ssid HiddenSecured -passphrase mock-secret-passphrase
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
	ssid := flag.String("ssid", "", "hidden network SSID to join (required)")
	passphrase := flag.String("passphrase", "", "passphrase for the hidden network (required)")
	flag.Parse()

	if *ssid == "" || *passphrase == "" {
		flag.Usage()
		log.Fatal("both -ssid and -passphrase are required")
	}

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

	// A hidden connection needs a passphrase, so register a credentials agent
	// before asking the station to connect. The agent is torn down on return.
	agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		Passphrase: func(context.Context, string) (string, error) { return *passphrase, nil },
	})
	if err != nil {
		log.Fatalf("register agent: %v", err)
	}
	defer func() { _ = agent.Unregister(ctx) }()

	stations, err := client.AllStations(ctx)
	if err != nil {
		log.Fatalf("list stations: %v", err)
	}
	if len(stations) == 0 {
		log.Fatal("no wireless stations available")
	}
	station := stations[0]

	fmt.Printf("connecting station %q to hidden network %q...\n", station.Name(), *ssid)
	if err := station.ConnectHiddenNetwork(ctx, *ssid); err != nil {
		log.Fatalf("connect to hidden network: %v", err)
	}
	fmt.Println("connected")
}
