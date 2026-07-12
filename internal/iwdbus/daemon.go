package iwdbus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/godbus/dbus/v5"
)

// IwdDaemonIface is the fully qualified D-Bus interface name for the iwd daemon.
const IwdDaemonIface = IwdService + ".Daemon"

// IwdDaemonPath is the D-Bus object path for the iwd daemon object.
const IwdDaemonPath dbus.ObjectPath = "/net/connman/iwd"

// AdapterRef is a lightweight reference to an adapter discovered by ObjectManager.
type AdapterRef struct {
	// Path is the canonical D-Bus object path for the adapter.
	Path dbus.ObjectPath

	// Name is the Adapter.Name property.
	Name string
}

// DeviceRef is a lightweight reference to a device discovered by ObjectManager.
type DeviceRef struct {
	// Path is the canonical D-Bus object path for the device.
	Path dbus.ObjectPath

	// Name is the Device.Name property.
	Name string
}

// BasicServiceSetRef is a lightweight reference to a basic service set (BSS)
// discovered by ObjectManager.
type BasicServiceSetRef struct {
	// Path is the canonical D-Bus object path for the BSS.
	Path dbus.ObjectPath

	// Address is the BasicServiceSet.Address (BSSID) property.
	Address string
}

// NetworkRef is a lightweight reference to a network discovered by ObjectManager.
type NetworkRef struct {
	// Path is the canonical D-Bus object path for the network.
	Path dbus.ObjectPath

	// Name is the Network.Name (SSID) property.
	Name string
}

// KnownNetworkRef is a lightweight reference to a known network discovered by
// ObjectManager.
type KnownNetworkRef struct {
	// Path is the canonical D-Bus object path for the known network.
	Path dbus.ObjectPath

	// Name is the KnownNetwork.Name property.
	Name string
}

// StationRef is a lightweight reference to a station discovered by ObjectManager.
// The Station interface exposes no Name property of its own, so Name is the Name
// of the device the station shares its object with (e.g. "wlan0").
type StationRef struct {
	// Path is the canonical D-Bus object path for the station (a device path).
	Path dbus.ObjectPath

	// Name is the co-located device's Name (best-effort; empty if unavailable).
	Name string
}

// Daemon provides typed access to the iwd daemon D-Bus interface.
type Daemon struct {
	conn  *dbus.Conn
	intro caller
}

// DaemonInfo contains the typed fields returned by Daemon.GetInfo.
type DaemonInfo struct {
	// Version is the raw daemon version string returned by iwd.
	Version string

	// StateDirectory is the raw daemon state directory returned by iwd.
	StateDirectory string

	// NetworkConfigurationEnabled is the raw network configuration flag returned by iwd.
	NetworkConfigurationEnabled bool
}

// NewDaemon returns a Daemon bound to the iwd daemon object.
//
// If the daemon interface is not implemented, NewDaemon returns (nil, nil).
func NewDaemon(ctx context.Context, conn *dbus.Conn) (*Daemon, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, IwdDaemonPath)
	if err != nil {
		return nil, WrapIntrospection(string(IwdDaemonPath), err)
	}
	if !intro.HasInterface(IwdDaemonIface) {
		_ = intro.Close()
		return nil, nil
	}

	return &Daemon{conn: conn, intro: caller(intro)}, nil
}

// GetInfo calls net.connman.iwd.Daemon.GetInfo and parses the result into a
// typed DaemonInfo struct.
func (d *Daemon) GetInfo(ctx context.Context) (*DaemonInfo, error) {
	if err := d.ensureInitialized(); err != nil {
		return nil, WrapIntrospection("Daemon.ensureInitialized", err)
	}

	body, err := d.intro.Call(ctx, IwdDaemonIface, "GetInfo")
	if err != nil {
		return nil, WrapMethod(IwdDaemonIface, "GetInfo", err)
	}
	if len(body) == 0 {
		return nil, WrapVariant("GetInfo", errors.New("empty DBus body"))
	}

	info, err := parseDaemonInfo(body[0])
	if err != nil {
		return nil, fmt.Errorf("Daemon.GetInfo: failed to parse daemon info: %w", err)
	}

	return info, nil
}

// GetVersion returns the Version field from GetInfo.
func (d *Daemon) GetVersion(ctx context.Context) (string, error) {
	if err := d.ensureInitialized(); err != nil {
		return "", WrapIntrospection("Daemon.ensureInitialized", err)
	}

	info, err := d.GetInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.Version, nil
}

// GetStateDirectory returns the StateDirectory field from GetInfo.
func (d *Daemon) GetStateDirectory(ctx context.Context) (string, error) {
	if err := d.ensureInitialized(); err != nil {
		return "", WrapIntrospection("Daemon.ensureInitialized", err)
	}

	info, err := d.GetInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.StateDirectory, nil
}

// IsNetworkConfigurationEnabled returns the NetworkConfigurationEnabled field
// from GetInfo.
func (d *Daemon) IsNetworkConfigurationEnabled(ctx context.Context) (bool, error) {
	if err := d.ensureInitialized(); err != nil {
		return false, WrapIntrospection("Daemon.ensureInitialized", err)
	}

	info, err := d.GetInfo(ctx)
	if err != nil {
		return false, err
	}
	return info.NetworkConfigurationEnabled, nil
}

// ensureInitialized verifies that d has been initialized by NewDaemon.
func (d *Daemon) ensureInitialized() error {
	if d.intro == nil {
		return ErrDaemonUninitialized
	}
	return nil
}

// parseDaemonMap binds raw D-Bus values to a DaemonInfo struct.
func parseDaemonMap(get func(k string) (interface{}, bool)) (*DaemonInfo, error) {
	var (
		version        string
		stateDir       string
		netConfEnabled bool
	)

	// Version (string, optional but expected).
	v, ok := get("Version")
	if ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, WrapVariant("Version", fmt.Errorf("expected string, got %T", v))
		}
		if s == "" {
			return nil, WrapVariant("Version", errors.New("info Version must not be empty"))
		}
		version = s
	}

	// StateDirectory (string)
	v, ok = get("StateDirectory")
	if ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, WrapVariant("StateDirectory", fmt.Errorf("expected string, got %T", v))
		}
		if s == "" {
			return nil, WrapVariant("StateDirectory", errors.New("StateDirectory must not be empty"))
		}
		stateDir = s
	}

	// NetworkConfigurationEnabled (bool)
	v, ok = get("NetworkConfigurationEnabled")
	if ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return nil, WrapVariant("NetworkConfigurationEnabled", fmt.Errorf("expected bool, got %T", v))
		}
		netConfEnabled = b
	}

	return &DaemonInfo{
		Version:                     version,
		StateDirectory:              stateDir,
		NetworkConfigurationEnabled: netConfEnabled,
	}, nil
}

// getRefs asks iwd's ObjectManager for managed objects and returns a reference
// for every object that implements iface, built by makeRef and sorted by object
// path. It is the shared enumeration skeleton behind GetAdapters, GetDevices,
// and the other Get* methods.
func getRefs[T any](
	ctx context.Context,
	conn *dbus.Conn,
	op, iface string,
	makeRef func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (T, error),
) ([]T, error) {
	if conn == nil {
		return nil, WrapConnection(op, ErrDaemonUninitialized)
	}

	objects, err := getManagedObjects(ctx, conn, IwdService)
	if err != nil {
		return nil, WrapIntrospection(DBusObjectManagerGetManagedObjects, err)
	}

	type entry struct {
		path dbus.ObjectPath
		ref  T
	}
	entries := make([]entry, 0, len(objects))
	for path, ifaces := range objects {
		if _, ok := ifaces[iface]; !ok {
			continue
		}
		// makeRef receives every interface on the object (not just iface) so a
		// ref can be enriched from a co-located interface -- e.g. a station reads
		// its Name from the Device interface it shares an object with.
		ref, err := makeRef(path, ifaces)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{path: path, ref: ref})
	}

	sort.Slice(entries, func(i, j int) bool {
		return string(entries[i].path) < string(entries[j].path)
	})

	refs := make([]T, len(entries))
	for i, e := range entries {
		refs[i] = e.ref
	}
	return refs, nil
}

// GetAdapters returns every object that implements net.connman.iwd.Adapter,
// along with its Name property.
func (d *Daemon) GetAdapters(ctx context.Context) ([]AdapterRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetAdapters", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetAdapters", IwdAdapterIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (AdapterRef, error) {
			name, err := objectNameFromManagedObject("adapter", path, ifaces[IwdAdapterIface])
			if err != nil {
				return AdapterRef{}, err
			}
			return AdapterRef{Path: path, Name: name}, nil
		})
}

// GetDevices returns every object that implements net.connman.iwd.Device, along
// with its Name property.
func (d *Daemon) GetDevices(ctx context.Context) ([]DeviceRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetDevices", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetDevices", IwdDeviceIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (DeviceRef, error) {
			name, err := objectNameFromManagedObject("device", path, ifaces[IwdDeviceIface])
			if err != nil {
				return DeviceRef{}, err
			}
			return DeviceRef{Path: path, Name: name}, nil
		})
}

// GetBasicServiceSets returns every object that implements
// net.connman.iwd.BasicServiceSet, along with its Address property. Unlike
// adapters and devices, a BSS has no Name property, so it is identified by its
// Address (BSSID).
func (d *Daemon) GetBasicServiceSets(ctx context.Context) ([]BasicServiceSetRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetBasicServiceSets", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetBasicServiceSets", IwdBasicServiceSetIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (BasicServiceSetRef, error) {
			address, err := bssAddressFromManagedObject(path, ifaces[IwdBasicServiceSetIface])
			if err != nil {
				return BasicServiceSetRef{}, err
			}
			return BasicServiceSetRef{Path: path, Address: address}, nil
		})
}

// bssAddressFromManagedObject validates a BSS object path and extracts its
// required Address property from a managed-object property map.
func bssAddressFromManagedObject(path dbus.ObjectPath, props map[string]dbus.Variant) (string, error) {
	if !path.IsValid() || !strings.HasPrefix(string(path), "/") {
		return "", WrapVariant("Path", fmt.Errorf("invalid basic service set path %q", path))
	}

	v, ok := props["Address"]
	if !ok {
		return "", WrapVariant("Address", fmt.Errorf("basic service set %s missing Address property", path))
	}

	s, ok := v.Value().(string)
	if !ok {
		return "", WrapVariant("Address", fmt.Errorf("expected string, got %T", v.Value()))
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", WrapVariant("Address", fmt.Errorf("basic service set Address was empty"))
	}
	return s, nil
}

// GetNetworks returns every object that implements net.connman.iwd.Network,
// along with its Name (SSID) property.
func (d *Daemon) GetNetworks(ctx context.Context) ([]NetworkRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetNetworks", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetNetworks", IwdNetworkIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (NetworkRef, error) {
			name, err := objectNameFromManagedObject("network", path, ifaces[IwdNetworkIface])
			if err != nil {
				return NetworkRef{}, err
			}
			return NetworkRef{Path: path, Name: name}, nil
		})
}

// GetKnownNetworks returns every object that implements
// net.connman.iwd.KnownNetwork, along with its Name property.
func (d *Daemon) GetKnownNetworks(ctx context.Context) ([]KnownNetworkRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetKnownNetworks", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetKnownNetworks", IwdKnownNetworkIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (KnownNetworkRef, error) {
			name, err := objectNameFromManagedObject("known network", path, ifaces[IwdKnownNetworkIface])
			if err != nil {
				return KnownNetworkRef{}, err
			}
			return KnownNetworkRef{Path: path, Name: name}, nil
		})
}

// GetStations returns every object that implements net.connman.iwd.Station. A
// station shares its object with a device, so it carries the co-located Device
// Name (e.g. "wlan0") as its own Name; the Station interface itself has none.
func (d *Daemon) GetStations(ctx context.Context) ([]StationRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetStations", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetStations", IwdStationIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (StationRef, error) {
			if !path.IsValid() || !strings.HasPrefix(string(path), "/") {
				return StationRef{}, WrapVariant("Path", fmt.Errorf("invalid station path %q", path))
			}
			// Best-effort: resolve the name from the shared Device interface. A
			// missing/odd Name leaves it empty rather than failing enumeration.
			return StationRef{Path: path, Name: stationNameFromDevice(ifaces)}, nil
		})
}

// AccessPointRef is a lightweight reference to an access point discovered by
// ObjectManager. Like a station it shares its object with a device, so Name is
// the co-located Device Name (e.g. "wlan1"), not the hosted network's SSID.
type AccessPointRef struct {
	// Path is the canonical D-Bus object path for the access point (a device
	// path).
	Path dbus.ObjectPath

	// Name is the co-located device's Name (best-effort; empty if unavailable).
	Name string
}

// GetAccessPoints enumerates devices currently in AP mode via ObjectManager (the
// objects exposing the AccessPoint interface). Like GetStations, each ref carries
// the co-located Device Name.
func (d *Daemon) GetAccessPoints(ctx context.Context) ([]AccessPointRef, error) {
	if d == nil {
		return nil, WrapConnection("Daemon.GetAccessPoints", ErrDaemonUninitialized)
	}
	return getRefs(ctx, d.conn, "Daemon.GetAccessPoints", IwdAccessPointIface,
		func(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) (AccessPointRef, error) {
			if !path.IsValid() || !strings.HasPrefix(string(path), "/") {
				return AccessPointRef{}, WrapVariant("Path", fmt.Errorf("invalid access point path %q", path))
			}
			return AccessPointRef{Path: path, Name: stationNameFromDevice(ifaces)}, nil
		})
}

// stationNameFromDevice extracts the Device Name co-located on a station's
// object, returning "" when absent or not a non-empty string.
func stationNameFromDevice(ifaces map[string]map[string]dbus.Variant) string {
	dev, ok := ifaces[IwdDeviceIface]
	if !ok {
		return ""
	}
	v, ok := dev["Name"]
	if !ok {
		return ""
	}
	s, ok := v.Value().(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// objectNameFromManagedObject validates an object path and extracts its required
// Name property from a managed-object property map. label names the object kind
// for error messages (e.g. "adapter", "device").
func objectNameFromManagedObject(label string, path dbus.ObjectPath, props map[string]dbus.Variant) (string, error) {
	if !path.IsValid() || !strings.HasPrefix(string(path), "/") {
		return "", WrapVariant("Path", fmt.Errorf("invalid %s path %q", label, path))
	}

	v, ok := props["Name"]
	if !ok {
		return "", WrapVariant("Name", fmt.Errorf("%s %s missing Name property", label, path))
	}

	s, ok := v.Value().(string)
	if !ok {
		return "", WrapVariant("Name", fmt.Errorf("expected string, got %T", v.Value()))
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", WrapVariant("Name", fmt.Errorf("%s Name was empty", label))
	}
	return s, nil
}

// parseDaemonInfo normalizes the D-Bus GetInfo reply into a typed struct.
func parseDaemonInfo(v interface{}) (*DaemonInfo, error) {
	switch raw := v.(type) {
	case map[string]dbus.Variant:
		return parseDaemonMap(func(k string) (interface{}, bool) {
			variant, exists := raw[k]
			if !exists {
				return nil, false
			}
			return variant.Value(), true
		})
	case map[string]interface{}:
		return parseDaemonMap(func(k string) (interface{}, bool) {
			value, exists := raw[k]
			if !exists {
				return nil, false
			}
			if variant, ok := value.(dbus.Variant); ok {
				return variant.Value(), true
			}
			return value, true
		})
	default:
		return nil, WrapVariant("GetInfo", fmt.Errorf("unexpected GetInfo payload type %T", raw))
	}
}
