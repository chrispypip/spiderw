// Command access-point-start-profile runs a device in AP mode from a stored iwd
// provisioning profile using AccessPoint.StartProfile. Unlike Start (see the
// access-point-start example), the SSID, security mode, and credentials are not
// passed on the call: they come from an AP profile file iwd reads from disk
// (typically /var/lib/iwd/ap/<name>.ap), so this call takes only the profile
// name. That is what lets a profile host modes beyond inline PSK. iwd returns an
// error matching spiderw.ErrNotFound when no such profile exists.
//
// By default it only PRINTS the current access-point status and changes nothing.
// Pass -profile to start from that profile, or -stop to tear it down - those
// CHANGE state, so they act only when named explicitly.
//
// The device must already be in AP mode (Device.SetMode "ap"); see the bring-up
// example for switching modes. It targets the system bus (real iwd) by default;
// pass -session for the mock (whose one seeded profile is named "MockProfile").
//
//	go run ./examples/access-point-start-profile
//	go run ./examples/access-point-start-profile -profile MyProfile
//	go run ./examples/access-point-start-profile -stop
//	go run ./examples/access-point-start-profile -session -profile MockProfile
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
	profile := flag.String("profile", "", "start the access point from this stored provisioning profile")
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

	case *profile != "":
		if err := ap.StartProfile(ctx, *profile); err != nil {
			if errors.Is(err, spiderw.ErrAlreadyExists) {
				log.Fatalf("access point %q already running; -stop it first", ap.Name())
			}
			if errors.Is(err, spiderw.ErrNotFound) {
				log.Fatalf("no stored AP profile named %q", *profile)
			}
			log.Fatalf("start access point %q from profile %q: %v", ap.Name(), *profile, err)
		}
		fmt.Printf("access point %q: started from profile %q\n", ap.Name(), *profile)
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
