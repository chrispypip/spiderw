// Command wsc-pin joins the first station to an access point via WSC (Wi-Fi
// Simple Configuration, formerly WPS) PIN mode, without typing a passphrase. It
// demonstrates Station.SimpleConfiguration + GeneratePin/StartPin.
//
// With no -pin, a PIN is generated and printed; enter it at your access point's
// WPS "add device by PIN" page within ~2 minutes. Pass -pin to use a specific
// PIN (e.g. one printed on the AP). This CHANGES network state: on success the
// station connects.
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/wsc-pin
//	go run ./examples/wsc-pin -pin=12345670
//	go run ./examples/wsc-pin -session
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
	pin := flag.String("pin", "", "WSC PIN to use; if empty, one is generated and printed")
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

	code := *pin
	if code == "" {
		generated, err := wsc.GeneratePin(ctx)
		if err != nil {
			log.Fatalf("generate PIN: %v", err)
		}
		code = generated
	}

	// Print before StartPin blocks, so it is clear enrollment is in progress and
	// what to do: the PIN must be entered at the access point's WPS page for the
	// exchange to complete. Most routers accept only the 8-digit PIN form.
	fmt.Printf("station %q: enter WSC PIN %s at your access point's WPS page; waiting for enrollment (Ctrl-C to cancel)...\n",
		station.Name(), code)

	if err := wsc.StartPin(ctx, code); err != nil {
		if ctx.Err() != nil {
			// Interrupted (Ctrl-C): ask iwd to abort the in-progress enrollment on
			// a fresh context rather than leaving it running.
			_ = wsc.Cancel(context.Background())
			log.Fatal("enrollment canceled")
		}
		log.Fatalf("pin enrollment: %v", err)
	}

	fmt.Printf("station %q: connected via WSC\n", station.Name())
}
