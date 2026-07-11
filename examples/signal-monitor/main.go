// Command signal-monitor watches the connected network's signal strength on the
// first station and prints a line each time the RSSI crosses one of the given
// dBm thresholds, until interrupted with Ctrl-C. It demonstrates the
// SignalLevelAgent (Station.MonitorSignalLevel) and changes nothing.
//
// iwd only reports signal levels for a *connected* network, so the station
// should already be connected (see the scan-and-connect example).
//
// The band reflects iwd's own averaged, driver-dependent RSSI, so transitions
// won't line up exactly with the thresholds you pass or with an instantaneous
// meter like "iw dev link" (expect a few dBm of difference, more so while
// moving), and a band the signal crosses quickly may be skipped. See
// SignalLevelConfig.Changed in the package docs for details.
//
// It targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/signal-monitor
//	go run ./examples/signal-monitor -session -thresholds=-60,-70,-80
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/chrispypip/spiderw"
)

func main() {
	session := flag.Bool("session", false, "use the session bus (iwd mock) instead of the system bus")
	thresholdArg := flag.String("thresholds", "-60,-70,-80", "comma-separated RSSI thresholds in dBm, highest first")
	flag.Parse()

	thresholds, err := parseThresholds(*thresholdArg)
	if err != nil {
		log.Fatalf("invalid -thresholds: %v", err)
	}

	// Cancelled on Ctrl-C (SIGINT) or SIGTERM, which unblocks the wait at the end
	// so the deferred Unregister runs.
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

	agent, err := station.MonitorSignalLevel(ctx, spiderw.SignalLevelConfig{
		Thresholds: thresholds,
		Changed: func(level int) {
			// iwd reports the band index, not the exact RSSI, so map it back to
			// the dBm range that band represents. 0 is the strongest band (above
			// the first threshold), len(thresholds) is the weakest (below the last).
			fmt.Printf("signal band %d of %d (%s)\n", level, len(thresholds), bandRange(level, thresholds))
		},
		Released: func() { fmt.Println("agent released by iwd") },
	})
	if err != nil {
		log.Fatalf("monitor signal level: %v", err)
	}
	// Unregister on a fresh context: the monitor context is already cancelled here.
	defer func() { _ = agent.Unregister(context.Background()) }()

	fmt.Printf("station %q: watching signal across thresholds %v dBm (Ctrl-C to stop)...\n",
		station.Name(), thresholds)
	<-ctx.Done()
	fmt.Println("\nstopping")
}

// bandRange describes, in dBm, the RSSI range a band index covers given the
// (descending) thresholds. For N thresholds there are N+1 bands: band 0 is above
// the first threshold, band N is below the last, and band i in between spans
// [thresholds[i], thresholds[i-1]).
func bandRange(level int, thresholds []int) string {
	n := len(thresholds)
	switch {
	case level <= 0:
		return fmt.Sprintf(">= %d dBm", thresholds[0])
	case level >= n:
		return fmt.Sprintf("< %d dBm", thresholds[n-1])
	default:
		return fmt.Sprintf("%d to %d dBm", thresholds[level], thresholds[level-1])
	}
}

// parseThresholds parses a comma-separated list of dBm thresholds into ints;
// range and descending-order validation is left to MonitorSignalLevel.
func parseThresholds(s string) ([]int, error) {
	fields := strings.Split(s, ",")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer dBm value", f)
		}
		out = append(out, v)
	}
	return out, nil
}
