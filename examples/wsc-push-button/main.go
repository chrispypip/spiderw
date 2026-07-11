// Command wsc-push-button joins the first station to an access point via WSC
// (Wi-Fi Simple Configuration, formerly WPS) PushButton mode, without typing a
// passphrase. It demonstrates Station.SimpleConfiguration + PushButton.
//
// Press the WPS button on your access point, then run this within its ~2-minute
// walk window. This CHANGES network state: on success the station connects to
// whichever access point is advertising PushButton mode. If more than one is,
// the target is ambiguous and enrollment fails with ErrWSCSessionOverlap.
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/wsc-push-button
//	go run ./examples/wsc-push-button -session
package main

import (
	"context"
	"errors"
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

	// Enrollment blocks until iwd reports the outcome; iwd enforces the WPS walk
	// time itself, so no client-side deadline is needed. Ctrl-C cancels the wait,
	// and we then ask iwd to abort the operation cleanly.
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

	wsc, err := station.SimpleConfiguration(ctx)
	if err != nil {
		log.Fatalf("WSC unavailable on %q: %v", station.Name(), err)
	}

	fmt.Printf("station %q: press the WPS button on your access point now; waiting for enrollment (Ctrl-C to cancel)...\n", station.Name())
	if err := wsc.PushButton(ctx); err != nil {
		if ctx.Err() != nil {
			// Interrupted (Ctrl-C): ask iwd to abort the in-progress enrollment on
			// a fresh context rather than leaving it running.
			_ = wsc.Cancel(context.Background())
			log.Fatal("enrollment canceled")
		}
		if errors.Is(err, spiderw.ErrWSCSessionOverlap) {
			log.Fatal("more than one access point is in WPS PushButton mode; wait and try again")
		}
		log.Fatalf("push-button enrollment: %v", err)
	}

	fmt.Printf("station %q: connected via WSC\n", station.Name())
}
