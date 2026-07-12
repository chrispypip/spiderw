// Command access-point-start runs a device in AP mode as a PSK-secured access
// point using AccessPoint.Start, where you supply the SSID and passphrase inline.
// For an access point whose SSID, security mode, and credentials come from a
// stored iwd provisioning profile, see the access-point-start-profile example.
//
// Note: on some FullMAC drivers — notably the Raspberry Pi's built-in brcmfmac
// chip — the kernel rejects the inline Start with a generic "failed starting"
// error even on an idle radio; access-point-start-profile is the reliable path
// there. See examples/README.md for details.
//
// By default it only PRINTS the current access-point status and changes nothing.
// Pass -ssid (with -passphrase) to start the AP, or -stop to tear it down — those
// CHANGE state, so they act only when named explicitly.
//
// The device must already be in AP mode (Device.SetMode "ap"); see the bring-up
// example for switching modes. It targets the system bus (real iwd) by default;
// pass -session for the mock.
//
//	go run ./examples/access-point-start
//	go run ./examples/access-point-start -ssid MyAP -passphrase 's3cretpass'
//	go run ./examples/access-point-start -stop
//	go run ./examples/access-point-start -session
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
	ssid := flag.String("ssid", "", "start a PSK access point advertising this SSID (requires -passphrase)")
	passphrase := flag.String("passphrase", "", "passphrase for -ssid (8-63 characters)")
	stop := flag.Bool("stop", false, "stop the running access point")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bus := spiderw.SystemBus
	if *session {
		bus = spiderw.SessionBus
	}

	client, err := spiderw.NewClient(ctx, bus)
	if err != nil {
		log.Fatalf("connect to iwd: %v", err)
	}
	defer func() { _ = client.Close() }()

	aps, err := client.AllAccessPoints(ctx)
	if err != nil {
		log.Fatalf("list access points: %v", err)
	}
	if len(aps) == 0 {
		log.Fatal("no access points available (put a device in AP mode first; see the bring-up example)")
	}
	ap := aps[0]

	switch {
	case *stop:
		if err := ap.Stop(ctx); err != nil {
			log.Fatalf("stop access point %q: %v", ap.Name(), err)
		}
		fmt.Printf("access point %q: stopped\n", ap.Name())

	case *ssid != "":
		if len(*passphrase) < 8 {
			log.Fatal("-passphrase must be 8-63 characters")
		}
		if err := ap.Start(ctx, *ssid, *passphrase); err != nil {
			if errors.Is(err, spiderw.ErrAlreadyExists) {
				log.Fatalf("access point %q already running; -stop it first", ap.Name())
			}
			log.Fatalf("start access point %q: %v", ap.Name(), err)
		}
		fmt.Printf("access point %q: started, hosting %q\n", ap.Name(), *ssid)
	}

	printStatus(ctx, ap)
}

// printStatus prints a one-block snapshot of the access point's state.
func printStatus(ctx context.Context, ap *spiderw.AccessPoint) {
	props, err := ap.Properties(ctx)
	if err != nil {
		log.Fatalf("read access-point properties: %v", err)
	}

	fmt.Printf("%s (%s)\n", ap.Name(), ap.Path())
	fmt.Printf("  Started:  %t\n", props.Started)
	// iwd reports Scanning (and the fields below) only while the AP is running.
	if !props.Started {
		return
	}
	fmt.Printf("  Scanning: %t\n", props.Scanning)
	if props.SSID != nil {
		fmt.Printf("  SSID:     %s\n", *props.SSID)
	}
	if props.Frequency != nil {
		fmt.Printf("  Frequency: %d MHz\n", *props.Frequency)
	}
	if len(props.PairwiseCiphers) > 0 {
		fmt.Printf("  Pairwise: %v\n", props.PairwiseCiphers)
	}
	if props.GroupCipher != nil {
		fmt.Printf("  Group:    %s\n", *props.GroupCipher)
	}
}
