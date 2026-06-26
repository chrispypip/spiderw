package mock

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// StartSignalFirehose starts background mock daemon, adapter, device, network,
// and known-network signal emitters.
func StartSignalFirehose() {
	go firehoseDaemonSignals()
	go firehoseAdapterSignals()
	go firehoseDeviceSignals()
	go firehoseNetworkSignals()
	go firehoseKnownNetworkSignals()
}

func firehoseDaemonSignals() {
	for i := 0; ; i++ {
		emitPropertiesChanged(
			daemonPath,
			iwdbus.IwdDaemonIface,
			map[string]dbus.Variant{
				"Version":                     dbus.MakeVariant(fmt.Sprintf("1.0.%d", i%50)),
				"NetworkConfigurationEnabled": dbus.MakeVariant(rand.Intn(2) == 0),
			},
			[]string{},
		)
		time.Sleep(4 * time.Millisecond)
	}
}

func firehoseAdapterSignals() {
	for {
		emitPropertiesChanged(
			adapterPath,
			iwdbus.IwdAdapterIface,
			map[string]dbus.Variant{
				"Powered":        dbus.MakeVariant(rand.Intn(2) == 0),
				"SupportedModes": dbus.MakeVariant([]string{"station", "ap", "ad-hoc"}),
			},
			[]string{},
		)
		time.Sleep(3 * time.Millisecond)
	}
}

func firehoseDeviceSignals() {
	modes := []string{"station", "ap", "ad-hoc"}
	for i := 0; ; i++ {
		emitPropertiesChanged(
			devicePath,
			iwdbus.IwdDeviceIface,
			map[string]dbus.Variant{
				"Powered": dbus.MakeVariant(rand.Intn(2) == 0),
				"Mode":    dbus.MakeVariant(modes[i%len(modes)]),
			},
			[]string{},
		)
		time.Sleep(3 * time.Millisecond)
	}
}

func firehoseNetworkSignals() {
	// Emit Connected toggles on the open network so subscribers see churn.
	networkPath := dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/open")
	for {
		emitPropertiesChanged(
			networkPath,
			iwdbus.IwdNetworkIface,
			map[string]dbus.Variant{
				"Connected": dbus.MakeVariant(rand.Intn(2) == 0),
			},
			[]string{},
		)
		time.Sleep(3 * time.Millisecond)
	}
}

func firehoseKnownNetworkSignals() {
	// Emit AutoConnect toggles on the first known network so subscribers see churn.
	knownNetworkPath := dbus.ObjectPath("/net/connman/iwd/known_networks/1")
	for {
		emitPropertiesChanged(
			knownNetworkPath,
			iwdbus.IwdKnownNetworkIface,
			map[string]dbus.Variant{
				"AutoConnect": dbus.MakeVariant(rand.Intn(2) == 0),
			},
			[]string{},
		)
		time.Sleep(3 * time.Millisecond)
	}
}
