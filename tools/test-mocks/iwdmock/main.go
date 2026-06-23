package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/tools/test-mocks/iwdmock/internal/mock"
)

var (
	firehoseSignalsFlag = flag.Bool("firehose-signals", false, "Emit DBus signals rapidly")
	omitOptionalsFlag   = flag.Bool("omit-optionals", false, "Set optional properties to nil")
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Assumes the session bus is used.
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatalf("[mock-iwd] ERROR: failed to connect session bus: %s", err)
	}
	defer func() { _ = conn.Close() }()

	flag.Parse()

	// Request the iwd name.
	reply, err := conn.RequestName(iwdbus.IwdService, dbus.NameFlagAllowReplacement|dbus.NameFlagReplaceExisting|dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatalf("[mock-iwd] ERROR: RequestName failed: %s", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("[mock-iwd] ERROR: could not acquire bus name %s (reply=%d)", iwdbus.IwdService, reply)
	}
	log.Printf("[mock-iwd] acquired name: %s", iwdbus.IwdService)

	// Register objects.
	if err := mock.ExportDaemon(conn); err != nil {
		log.Fatalf("[mock-iwd] ERROR: exportDaemon: %s", err)
	}

	if *omitOptionalsFlag {
		if err := mock.ExportAdapter(conn, nil, nil); err != nil {
			log.Fatalf("[mock-iwd] ERROR: exportAdapter: %s", err)
		}
	} else {
		model := "MockModel"
		vendor := "MockVendor"
		if err := mock.ExportAdapter(conn, &model, &vendor); err != nil {
			log.Fatalf("[mock-iwd] ERROR: exportAdapter: %s", err)
		}
	}

	if err := mock.ExportDevice(conn); err != nil {
		log.Fatalf("[mock-iwd] ERROR: exportDevice: %s", err)
	}

	if err := mock.ExportBasicServiceSet(conn); err != nil {
		log.Fatalf("[mock-iwd] ERROR: exportBasicServiceSet: %s", err)
	}

	if err := mock.ExportObjectManager(conn); err != nil {
		log.Fatalf("[mock-iwd] ERROR: exportObjectManager: %s", err)
	}

	if *firehoseSignalsFlag {
		fmt.Println("[mock-iwd] fire hose mode ENABLED")
		mock.StartSignalFirehose()
	}

	time.Sleep(25 * time.Millisecond)

	fmt.Println("[mock-iwd] READY")
	log.Println("[mock-iwd] running. Press Ctrl+C to exit.")
	_ = os.Stdout.Sync()

	// Wait forever (until Ctrl+C).
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	log.Println("[mock-iwd] shutting down")

	if reply, err := conn.ReleaseName(iwdbus.IwdService); err != nil {
		log.Printf("[mock-iwd] WARNING: failed to release name %s (reply=%d)", iwdbus.IwdService, reply)
	}
}
