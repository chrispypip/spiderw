// Command monitor watches the first station and prints its state, scanning,
// connected-network, and associated-access-point changes live, until interrupted
// with Ctrl-C. It demonstrates the subscription API and how signals flow through
// spiderw. It changes nothing.
//
// The access-point subscription is how a roam is observed: the station moves
// between access points of the same network, so the BSS changes while the state
// stays connected and the network does not change at all. A nil value means the
// object is gone - iwd signals that by invalidating the property.
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/monitor           # real iwd
//	go run ./examples/monitor -session  # iwd mock
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrispypip/spiderw"
)

func main() {
	session := flag.Bool("session", false, "use the session bus (iwd mock) instead of the system bus")
	flag.Parse()

	// The context is cancelled on Ctrl-C (SIGINT) or SIGTERM, which unblocks the
	// wait at the end and lets the deferred cleanup run.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
		log.Fatal("no wireless stations available")
	}
	station := stations[0]

	state, err := station.State(ctx)
	if err != nil {
		log.Fatalf("read state: %v", err)
	}
	scanning, err := station.Scanning(ctx)
	if err != nil {
		log.Fatalf("read scanning: %v", err)
	}
	fmt.Printf("station %q: state=%s scanning=%t\n", station.Name(), state, scanning)

	unsubState, err := station.SubscribeStateChanged(ctx, func(s spiderw.StationState) {
		fmt.Printf("state -> %s\n", s)
	})
	if err != nil {
		log.Fatalf("subscribe state: %v", err)
	}
	defer func() { _ = unsubState.Unsubscribe() }()

	unsubScan, err := station.SubscribeScanningChanged(ctx, func(scanning bool) {
		fmt.Printf("scanning -> %t\n", scanning)
	})
	if err != nil {
		log.Fatalf("subscribe scanning: %v", err)
	}
	defer func() { _ = unsubScan.Unsubscribe() }()

	// Which network we are on. A nil path means disconnected: iwd signals "gone"
	// by invalidating the property, and spiderw delivers that as nil.
	unsubNet, err := station.SubscribeConnectedNetworkChanged(ctx, func(path *string) {
		if path == nil {
			fmt.Println("network -> none (disconnected)")
			return
		}
		fmt.Printf("network -> %s\n", *path)
	})
	if err != nil {
		log.Fatalf("subscribe connected network: %v", err)
	}
	defer func() { _ = unsubNet.Unsubscribe() }()

	// The access point we are associated with. This is the only way to observe a
	// roam: the station moves between APs of the same network, so the BSS changes
	// while the state stays connected and the network does not change at all.
	unsubAP, err := station.SubscribeConnectedAccessPointChanged(ctx, func(path *string) {
		if path == nil {
			fmt.Println("access-point -> none (not associated)")
			return
		}
		fmt.Printf("access-point -> %s\n", *path)
	})
	if err != nil {
		log.Fatalf("subscribe connected access point: %v", err)
	}
	defer func() { _ = unsubAP.Unsubscribe() }()

	fmt.Println("watching for changes (Ctrl-C to stop)...")
	<-ctx.Done()
	fmt.Println("\nstopping")
}
