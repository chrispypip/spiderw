// Command bring-up takes a wireless device from cold to ready for station work:
// it powers on the device's adapter, powers on the device, switches it to
// station mode, then scans to confirm the station is usable. This is the setup
// that the other station examples (scan-and-connect, monitor) assume has already
// happened.
//
// It changes device state (power and mode), so it acts on the first device by
// default; pass -device <name> to choose one.
//
// Targets the system bus (real iwd) by default; pass -session for the mock.
//
//	go run ./examples/bring-up                     # real iwd, first device
//	go run ./examples/bring-up -device wlan0
//	go run ./examples/bring-up -session -device wlan0
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
	deviceName := flag.String("device", "", "device to bring up by name (default: first available)")
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

	device, err := pickDevice(ctx, client, *deviceName)
	if err != nil {
		log.Fatalf("select device: %v", err)
	}
	name, err := device.Name(ctx)
	if err != nil {
		log.Fatalf("read device name: %v", err)
	}
	fmt.Printf("bringing up device %q\n", name)

	// 1. Power on the adapter the device belongs to: a device cannot come up
	//    while its adapter is powered down.
	adapterPath, err := device.Adapter(ctx)
	if err != nil {
		log.Fatalf("read device adapter: %v", err)
	}
	adapter, err := client.Adapter(ctx, adapterPath)
	if err != nil {
		log.Fatalf("open adapter: %v", err)
	}
	if err := adapter.SetPowered(ctx, true); err != nil {
		log.Fatalf("power on adapter: %v", err)
	}
	fmt.Println("  adapter powered on")

	// 2. Power on the device itself.
	if err := device.SetPowered(ctx, true); err != nil {
		log.Fatalf("power on device: %v", err)
	}
	fmt.Println("  device powered on")

	// 3. Switch it to station mode. iwd only exposes the Station interface (Scan,
	//    Connect, ...) on a device whose mode is station.
	if err := device.SetMode(ctx, spiderw.ModeStation); err != nil {
		log.Fatalf("set station mode: %v", err)
	}
	fmt.Println("  mode set to station")

	// 4. The mode change is asynchronous, so wait for the station interface to
	//    become usable before driving it.
	station, err := waitForStation(ctx, client, device.Path())
	if err != nil {
		log.Fatalf("wait for station: %v", err)
	}

	// 5. Prove it works: scan and list what the now-ready station can see.
	fmt.Println("  scanning...")
	if err := station.Scan(ctx); err != nil {
		log.Fatalf("scan: %v", err)
	}
	waitForScan(ctx, station)

	networks, err := station.OrderedNetworks(ctx)
	if err != nil {
		log.Fatalf("read scan results: %v", err)
	}
	fmt.Printf("\nstation %q is up and sees %d networks:\n", station.Name(), len(networks))
	for _, n := range networks {
		fmt.Printf("  %-20s %6.1f dBm\n", n.Name, n.SignalStrength)
	}
}

func pickDevice(ctx context.Context, client *spiderw.Client, name string) (*spiderw.Device, error) {
	devices, err := client.AllDevices(ctx)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, errors.New("no wireless devices found")
	}
	if name == "" {
		return devices[0], nil
	}
	for _, d := range devices {
		n, err := d.Name(ctx)
		if err != nil {
			continue
		}
		if n == name {
			return d, nil
		}
	}
	return nil, fmt.Errorf("no device named %q", name)
}

// waitForStation returns a Station handle for path once its interface responds,
// polling because SetMode takes effect asynchronously.
func waitForStation(ctx context.Context, client *spiderw.Client, path string) (*spiderw.Station, error) {
	station, err := client.Station(ctx, path)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := station.State(ctx); err == nil {
			return station, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("station interface did not become ready in time")
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// waitForScan blocks until the station reports it has stopped scanning, or a
// timeout elapses. Scan itself only schedules the scan.
func waitForScan(ctx context.Context, station *spiderw.Station) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		scanning, err := station.Scanning(ctx)
		if err != nil || !scanning {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}
