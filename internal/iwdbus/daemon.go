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

// GetAdapters asks iwd's ObjectManager for managed objects and returns every
// object that implements net.connman.iwd.Adapter, along with its Name property.
func (d *Daemon) GetAdapters(ctx context.Context) ([]AdapterRef, error) {
	const op = "Daemon.GetAdapters"

	if d == nil || d.conn == nil {
		return nil, WrapConnection(op, ErrDaemonUninitialized)
	}

	objects, err := getManagedObjects(ctx, d.conn, IwdService)
	if err != nil {
		return nil, WrapIntrospection(DBusObjectManagerGetManagedObjects, err)
	}
	refs := make([]AdapterRef, 0, len(objects))
	for path, ifaces := range objects {
		props, ok := ifaces[IwdAdapterIface]
		if !ok {
			continue
		}

		name, err := adapterNameFromManagedObject(path, props)
		if err != nil {
			return nil, err
		}

		refs = append(refs, AdapterRef{Path: path, Name: name})
	}

	sort.Slice(refs, func(i, j int) bool {
		return string(refs[i].Path) < string(refs[j].Path)
	})

	return refs, nil
}

func adapterNameFromManagedObject(path dbus.ObjectPath, props map[string]dbus.Variant) (string, error) {
	return objectNameFromManagedObject("adapter", path, props)
}

// GetDevices asks iwd's ObjectManager for managed objects and returns every
// object that implements net.connman.iwd.Device, along with its Name property.
func (d *Daemon) GetDevices(ctx context.Context) ([]DeviceRef, error) {
	const op = "Daemon.GetDevices"

	if d == nil || d.conn == nil {
		return nil, WrapConnection(op, ErrDaemonUninitialized)
	}

	objects, err := getManagedObjects(ctx, d.conn, IwdService)
	if err != nil {
		return nil, WrapIntrospection(DBusObjectManagerGetManagedObjects, err)
	}
	refs := make([]DeviceRef, 0, len(objects))
	for path, ifaces := range objects {
		props, ok := ifaces[IwdDeviceIface]
		if !ok {
			continue
		}

		name, err := objectNameFromManagedObject("device", path, props)
		if err != nil {
			return nil, err
		}

		refs = append(refs, DeviceRef{Path: path, Name: name})
	}

	sort.Slice(refs, func(i, j int) bool {
		return string(refs[i].Path) < string(refs[j].Path)
	})

	return refs, nil
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
