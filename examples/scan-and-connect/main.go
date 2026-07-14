// Command scan-and-connect runs the full "join a network" flow against the
// first station: trigger a scan, wait for it to finish, list the visible
// networks by signal strength, and - if -ssid is given - connect to one,
// supplying a passphrase through a credentials agent when needed.
//
// With no -ssid it only scans and lists (read-only). Passing -ssid is the
// explicit opt-in to actually connect, which changes state on a real system.
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/scan-and-connect -session
//	go run ./examples/scan-and-connect -session -ssid OpenNet
//	go run ./examples/scan-and-connect -session -ssid SecuredNet -passphrase mock-secret-passphrase
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chrispypip/spiderw"
)

func main() {
	session := flag.Bool("session", false, "use the session bus (iwd mock) instead of the system bus")
	ssid := flag.String("ssid", "", "network to connect to (omit to only scan and list)")
	passphrase := flag.String("passphrase", "", "passphrase for a secured network")
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

	stations, err := client.AllStations(ctx)
	if err != nil {
		log.Fatalf("list stations: %v", err)
	}
	if len(stations) == 0 {
		log.Fatal("no wireless stations available (is an adapter in station mode?)")
	}
	station := stations[0]
	fmt.Printf("using station %q\n", station.Name())

	if err := scanAndWait(ctx, station); err != nil {
		log.Fatalf("scan: %v", err)
	}

	networks, err := station.OrderedNetworks(ctx)
	if err != nil {
		log.Fatalf("read scan results: %v", err)
	}
	fmt.Printf("\nvisible networks (%d), strongest first:\n", len(networks))
	for _, n := range networks {
		fmt.Printf("  %-20s %6.1f dBm\n", n.Name, n.SignalStrength)
	}

	if *ssid == "" {
		fmt.Println("\n(no -ssid given; not connecting)")
		return
	}

	if err := connect(ctx, client, *ssid, *passphrase); err != nil {
		log.Fatalf("connect to %q: %v", *ssid, err)
	}
	fmt.Printf("\nconnected to %q\n", *ssid)
}

// scanAndWait triggers a scan and blocks until the station reports it has
// stopped scanning (or a timeout). Scan itself only schedules the scan.
func scanAndWait(ctx context.Context, station *spiderw.Station) error {
	fmt.Println("scanning...")
	if err := station.Scan(ctx); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		scanning, err := station.Scanning(ctx)
		if err != nil {
			return err
		}
		if !scanning {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New("timed out waiting for scan to finish")
}

// connect finds the network handle for ssid and connects to it. For a secured
// network that iwd does not already know, it first registers a credentials
// agent whose passphrase callback returns the supplied passphrase; the agent is
// unregistered on return.
func connect(ctx context.Context, client *spiderw.Client, ssid, passphrase string) error {
	networks, err := client.AllNetworks(ctx)
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	var target *spiderw.Network
	for _, n := range networks {
		name, err := n.Name(ctx)
		if err != nil {
			continue
		}
		if name == ssid {
			target = n
			break
		}
	}
	if target == nil {
		return fmt.Errorf("network %q not found in scan results", ssid)
	}

	if passphrase != "" {
		agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
			Passphrase: func(context.Context, string) (string, error) { return passphrase, nil },
		})
		if err != nil {
			return fmt.Errorf("register agent: %w", err)
		}
		defer func() { _ = agent.Unregister(ctx) }()
	}

	// Open and already-known networks connect without an agent. Connecting to a
	// secured network that is not already known fails with an error matching
	// spiderw.ErrNoAgent unless an agent is registered.
	if err := target.Connect(ctx); err != nil {
		if errors.Is(err, spiderw.ErrNoAgent) {
			return errors.New("this network needs a passphrase; pass -passphrase")
		}
		return err
	}
	return nil
}
