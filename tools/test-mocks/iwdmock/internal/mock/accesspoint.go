package mock

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// The mock AP-mode device (wlan1) reports a running access point, so property
// reads resolve to a hosted SSID and negotiated ciphers. Integration tests
// assert against these literals.
const (
	accessPointHostedSSID  = "MockAP"
	accessPointFrequency   = uint32(5180)
	accessPointGroupCipher = "CCMP"

	// accessPointProfileSSID is the one stored profile StartProfile accepts; any
	// other name is rejected NotFound.
	accessPointProfileSSID = "MockProfile"
)

// accessPointPairwiseCiphers is the mock's negotiated unicast cipher list.
var accessPointPairwiseCiphers = []string{"CCMP"}

// accessPointOrderedNetworks is the seeded AP scan result: neighbor networks the
// hosted AP heard, strongest first. godbus marshals a slice of these dicts to
// aa{sv} (Name s / SignalStrength n / Type s), matching iwd's on-wire shape (the
// security key is "Type", not "Security" as iwd's own docs claim).
var accessPointOrderedNetworks = []map[string]dbus.Variant{
	{
		"Name":           dbus.MakeVariant("OpenNet"),
		"SignalStrength": dbus.MakeVariant(int16(-6000)),
		"Type":           dbus.MakeVariant("open"),
	},
	{
		"Name":           dbus.MakeVariant("SecuredNet"),
		"SignalStrength": dbus.MakeVariant(int16(-7200)),
		"Type":           dbus.MakeVariant("psk"),
	},
	{
		// A neighbor whose security iwd could not classify: an empty Type field.
		// Defensive — real iwd classifies scanned networks into a known type, but
		// the client must surface an unclassifiable one as unknown rather than
		// failing the whole list.
		"Name":           dbus.MakeVariant("MysteryNet"),
		"SignalStrength": dbus.MakeVariant(int16(-8100)),
		"Type":           dbus.MakeVariant(""),
	},
}

// accessPointExported reports whether the AccessPoint interface should be
// exported on the AP-mode device and advertised in introspection/ObjectManager.
func accessPointExported() bool {
	return !*omitAccessPointFlag
}

// AccessPoint is the mock net.connman.iwd.AccessPoint dispatch object. It lives
// on the AP-mode device's path; property reads are served by the Device's
// Properties handler, while method calls land here. Method calls delegate back
// to the Device (which owns the AP state) via distinctly named helpers, so they
// do not collide with the Station-mode Device methods of the same D-Bus name
// (Scan, GetOrderedNetworks).
type AccessPoint struct {
	device *Device
}

// Start implements net.connman.iwd.AccessPoint.Start.
func (a *AccessPoint) Start(ssid, psk string) *dbus.Error { return a.device.apStart(ssid, psk) }

// StartProfile implements net.connman.iwd.AccessPoint.StartProfile.
func (a *AccessPoint) StartProfile(ssid string) *dbus.Error { return a.device.apStartProfile(ssid) }

// Stop implements net.connman.iwd.AccessPoint.Stop.
func (a *AccessPoint) Stop() *dbus.Error { return a.device.apStop() }

// Scan implements net.connman.iwd.AccessPoint.Scan.
func (a *AccessPoint) Scan() *dbus.Error { return a.device.apScan() }

// GetOrderedNetworks implements net.connman.iwd.AccessPoint.GetOrderedNetworks.
func (a *AccessPoint) GetOrderedNetworks() ([]map[string]dbus.Variant, *dbus.Error) {
	return a.device.apOrderedNetworks()
}

// buildAccessPointPropertyMap returns the mock AccessPoint interface properties.
// Started is the only always-present property. Scanning belongs with the rest of
// the optionals (Name/Frequency/ciphers): all of them appear only while the AP is
// running, mirroring iwd. A stopped AP therefore reports Started and nothing else.
func (d *Device) buildAccessPointPropertyMap() map[string]dbus.Variant {
	d.apMu.Lock()
	defer d.apMu.Unlock()
	props := map[string]dbus.Variant{
		"Started": dbus.MakeVariant(d.APStarted),
	}
	if d.APStarted {
		// Scanning is listed here, with the optionals, even though iwd's docs call it
		// mandatory: on hardware a stopped AP reports only Started, so the client must
		// tolerate an absent Scanning (collapsing it to false).
		props["Scanning"] = dbus.MakeVariant(d.APScanning)
		props["Name"] = dbus.MakeVariant(d.APName)
		props["Frequency"] = dbus.MakeVariant(d.APFrequency)
		props["PairwiseCiphers"] = dbus.MakeVariant(d.APPairwiseCiphers)
		props["GroupCipher"] = dbus.MakeVariant(d.APGroupCipher)
	}
	return props
}

// apStart implements the AccessPoint.Start behavior: bring up a PSK AP with the
// given SSID. A start while already running is rejected AlreadyExists.
func (d *Device) apStart(ssid, psk string) *dbus.Error {
	if !d.HasAccessPoint {
		return dbus.MakeFailedError(fmt.Errorf("device has no access point interface"))
	}
	if ssid == "" || len(psk) < 8 {
		return dbus.NewError(iwdbus.IwdErrorInvalidArguments, []interface{}{"invalid ssid or passphrase"})
	}
	d.apMu.Lock()
	if d.APStarted {
		d.apMu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorAlreadyExists, []interface{}{"access point already started"})
	}
	d.startAPLocked(ssid)
	d.apMu.Unlock()
	d.emitAccessPointStarted(ssid)
	return nil
}

// apStartProfile implements AccessPoint.StartProfile: start from a stored profile.
// Only accessPointProfileSSID is known; anything else is NotFound.
func (d *Device) apStartProfile(ssid string) *dbus.Error {
	if !d.HasAccessPoint {
		return dbus.MakeFailedError(fmt.Errorf("device has no access point interface"))
	}
	d.apMu.Lock()
	if d.APStarted {
		d.apMu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorAlreadyExists, []interface{}{"access point already started"})
	}
	if ssid != accessPointProfileSSID {
		d.apMu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorNotFound, []interface{}{"no such profile"})
	}
	d.startAPLocked(ssid)
	d.apMu.Unlock()
	d.emitAccessPointStarted(ssid)
	return nil
}

// startAPLocked populates the running-AP state. Callers hold apMu.
func (d *Device) startAPLocked(ssid string) {
	d.APStarted = true
	d.APName = ssid
	d.APFrequency = accessPointFrequency
	d.APPairwiseCiphers = accessPointPairwiseCiphers
	d.APGroupCipher = accessPointGroupCipher
}

// emitAccessPointStarted emits the property changes for a freshly started AP.
func (d *Device) emitAccessPointStarted(ssid string) {
	d.emitAccessPointPropertiesChanged(map[string]dbus.Variant{
		"Started":         dbus.MakeVariant(true),
		"Scanning":        dbus.MakeVariant(false),
		"Name":            dbus.MakeVariant(ssid),
		"Frequency":       dbus.MakeVariant(accessPointFrequency),
		"PairwiseCiphers": dbus.MakeVariant(accessPointPairwiseCiphers),
		"GroupCipher":     dbus.MakeVariant(accessPointGroupCipher),
	})
}

// apStop implements AccessPoint.Stop: tear down the AP and clear the optional
// properties (invalidated so subscribers re-read them as absent).
func (d *Device) apStop() *dbus.Error {
	if !d.HasAccessPoint {
		return dbus.MakeFailedError(fmt.Errorf("device has no access point interface"))
	}
	d.apMu.Lock()
	d.APStarted = false
	d.APName = ""
	d.APFrequency = 0
	d.APPairwiseCiphers = nil
	d.APGroupCipher = ""
	d.apMu.Unlock()
	emitPropertiesChanged(d.Path, iwdbus.IwdAccessPointIface,
		map[string]dbus.Variant{"Started": dbus.MakeVariant(false)},
		[]string{"Scanning", "Name", "Frequency", "PairwiseCiphers", "GroupCipher"})
	return nil
}

// apScan models the async AP scan: flip Scanning true, then false after
// scanDuration, emitting on each transition. A scan on a stopped AP is rejected
// with NotAvailable ("Operation not available", confirmed on hardware) — an AP
// that is not running has no radio configured to survey with. A scan already in
// progress is rejected with InProgress (iwd's dbus_error_busy name).
func (d *Device) apScan() *dbus.Error {
	if !d.HasAccessPoint {
		return dbus.MakeFailedError(fmt.Errorf("device has no access point interface"))
	}
	d.apMu.Lock()
	if !d.APStarted {
		d.apMu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorNotAvailable, []interface{}{"Operation not available"})
	}
	if d.APScanning {
		d.apMu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorInProgress, []interface{}{"scan already in progress"})
	}
	d.APScanning = true
	d.apMu.Unlock()
	d.emitAccessPointPropertiesChanged(map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)})

	go func() {
		time.Sleep(scanDuration)
		d.apMu.Lock()
		d.APScanning = false
		d.apMu.Unlock()
		d.emitAccessPointPropertiesChanged(map[string]dbus.Variant{"Scanning": dbus.MakeVariant(false)})
	}()
	return nil
}

// apOrderedNetworks returns the seeded AP scan result as aa{sv}. Reading scan
// results from a stopped AP is rejected with NotAvailable ("Operation not
// available", confirmed on hardware), exactly as Scan is: a stopped AP has no
// scan state to report.
func (d *Device) apOrderedNetworks() ([]map[string]dbus.Variant, *dbus.Error) {
	if !d.HasAccessPoint {
		return nil, dbus.MakeFailedError(fmt.Errorf("device has no access point interface"))
	}
	d.apMu.Lock()
	started := d.APStarted
	d.apMu.Unlock()
	if !started {
		return nil, dbus.NewError(iwdbus.IwdErrorNotAvailable, []interface{}{"Operation not available"})
	}
	return accessPointOrderedNetworks, nil
}

// emitAccessPointPropertiesChanged emits a PropertiesChanged signal on the
// AccessPoint interface for the device path, so subscription tests observe it.
func (d *Device) emitAccessPointPropertiesChanged(changed map[string]dbus.Variant) {
	emitPropertiesChanged(d.Path, iwdbus.IwdAccessPointIface, changed, []string{})
}
