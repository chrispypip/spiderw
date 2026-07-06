package spiderw

import (
	"context"
	"fmt"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// AdapterRef is a lightweight reference to an adapter discovered by the iwd
// daemon.
type AdapterRef struct {
	// Path is the canonical D-Bus object path for the adapter.
	Path string

	// Name is the adapter's human-friendly Name property.
	Name string
}

// DeviceRef is a lightweight reference to a device discovered by the iwd daemon.
type DeviceRef struct {
	// Path is the canonical D-Bus object path for the device.
	Path string

	// Name is the device's human-friendly Name property.
	Name string
}

// BasicServiceSetRef is a lightweight reference to a basic service set (BSS)
// discovered by the iwd daemon.
type BasicServiceSetRef struct {
	// Path is the canonical D-Bus object path for the BSS.
	Path string

	// Address is the BSS's hardware (BSSID) address.
	Address string
}

// NetworkRef is a lightweight reference to a network discovered by the iwd
// daemon.
type NetworkRef struct {
	// Path is the canonical D-Bus object path for the network.
	Path string

	// Name is the network's SSID.
	Name string
}

// KnownNetworkRef is a lightweight reference to a known network discovered by the
// iwd daemon.
type KnownNetworkRef struct {
	// Path is the canonical D-Bus object path for the known network.
	Path string

	// Name is the known network's name (usually the SSID).
	Name string
}

// StationRef is a lightweight reference to a station discovered by the iwd
// daemon. A station shares its object with a device and has no Name of its own,
// so Name is the co-located device's Name (e.g. "wlan0"), resolved best-effort.
// Resolve the handle with Client.Station.
type StationRef struct {
	// Path is the canonical D-Bus object path for the station (a device path).
	Path string

	// Name is the co-located device's Name (best-effort; empty if unavailable).
	Name string
}

// DaemonInfo is the public API view of the iwd daemon metadata.
//
// This intentionally mirrors core.DaemonInfo but is separate to avoid leaking
// internal types into the API surface. Future evolution of the internal/core
// types will not affect public clients.
type DaemonInfo struct {
	// Version is the iwd daemon version string.
	Version string

	// StateDirectory is the daemon's persistent state directory.
	StateDirectory string

	// NetworkConfigurationEnabled reports whether iwd manages network configuration.
	NetworkConfigurationEnabled bool
}

// String returns a human-readable multiline representation of the daemon info.
func (d *DaemonInfo) String() string {
	if d == nil {
		return "<nil>"
	}

	return fmt.Sprintf(
		"Version: %s\nStateDirectory: %s\nNetworkConfigurationEnabled: %t",
		d.Version,
		d.StateDirectory,
		d.NetworkConfigurationEnabled,
	)
}

// Daemon provides high-level operations for the singleton iwd daemon object.
type Daemon struct {
	core core.DaemonIface
}

func newDaemon(c core.DaemonIface) *Daemon {
	if c == nil {
		return nil
	}
	return &Daemon{core: c}
}

func (d *Daemon) coreDaemon(ctx context.Context, op string) (core.DaemonIface, error) {
	if d == nil || d.core == nil {
		logging.FromContext(ctx).Error(ctx, "daemon wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return d.core, nil
}

// Info returns the daemon metadata reported by iwd.
func (d *Daemon) Info(ctx context.Context) (*DaemonInfo, error) {
	return delegate(ctx, "Daemon.Info", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) (*DaemonInfo, error) {
		ci, err := c.Info(ctx)
		if err != nil {
			return nil, err
		}
		return &DaemonInfo{
			Version:                     ci.Version,
			StateDirectory:              ci.StateDirectory,
			NetworkConfigurationEnabled: ci.NetworkConfigurationEnabled,
		}, nil
	})
}

// Version returns the iwd daemon version.
func (d *Daemon) Version(ctx context.Context) (string, error) {
	return delegate(ctx, "Daemon.Version", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) (string, error) {
		return c.Version(ctx)
	})
}

// StateDirectory returns the daemon's persistent state directory.
func (d *Daemon) StateDirectory(ctx context.Context) (string, error) {
	return delegate(ctx, "Daemon.StateDirectory", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) (string, error) {
		return c.StateDirectory(ctx)
	})
}

// NetworkConfigurationEnabled reports whether iwd manages network configuration.
func (d *Daemon) NetworkConfigurationEnabled(ctx context.Context) (bool, error) {
	return delegate(ctx, "Daemon.NetworkConfigurationEnabled", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) (bool, error) {
		return c.NetworkConfigurationEnabled(ctx)
	})
}

// Adapters returns lightweight references to the adapters currently exposed by iwd.
func (d *Daemon) Adapters(ctx context.Context) ([]AdapterRef, error) {
	return delegate(ctx, "Daemon.Adapters", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) ([]AdapterRef, error) {
		refs, err := c.Adapters(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]AdapterRef, 0, len(refs))
		for _, r := range refs {
			out = append(out, AdapterRef{Path: r.Path, Name: r.Name})
		}
		return out, nil
	})
}

// Devices returns lightweight references to the devices currently exposed by iwd.
func (d *Daemon) Devices(ctx context.Context) ([]DeviceRef, error) {
	return delegate(ctx, "Daemon.Devices", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) ([]DeviceRef, error) {
		refs, err := c.Devices(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]DeviceRef, 0, len(refs))
		for _, r := range refs {
			out = append(out, DeviceRef{Path: r.Path, Name: r.Name})
		}
		return out, nil
	})
}

// Stations returns lightweight references to the stations (station-mode devices)
// currently exposed by iwd. Resolve one with Client.Station.
func (d *Daemon) Stations(ctx context.Context) ([]StationRef, error) {
	return delegate(ctx, "Daemon.Stations", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) ([]StationRef, error) {
		refs, err := c.Stations(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]StationRef, 0, len(refs))
		for _, r := range refs {
			out = append(out, StationRef{Path: r.Path, Name: r.Name})
		}
		return out, nil
	})
}

// BasicServiceSets returns lightweight references to the basic service sets
// currently exposed by iwd.
func (d *Daemon) BasicServiceSets(ctx context.Context) ([]BasicServiceSetRef, error) {
	return delegate(ctx, "Daemon.BasicServiceSets", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) ([]BasicServiceSetRef, error) {
		refs, err := c.BasicServiceSets(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]BasicServiceSetRef, 0, len(refs))
		for _, r := range refs {
			out = append(out, BasicServiceSetRef{Path: r.Path, Address: r.Address})
		}
		return out, nil
	})
}

// Networks returns lightweight references to the networks currently exposed by
// iwd.
func (d *Daemon) Networks(ctx context.Context) ([]NetworkRef, error) {
	return delegate(ctx, "Daemon.Networks", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) ([]NetworkRef, error) {
		refs, err := c.Networks(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]NetworkRef, 0, len(refs))
		for _, r := range refs {
			out = append(out, NetworkRef{Path: r.Path, Name: r.Name})
		}
		return out, nil
	})
}

// KnownNetworks returns lightweight references to the known networks currently
// exposed by iwd.
func (d *Daemon) KnownNetworks(ctx context.Context) ([]KnownNetworkRef, error) {
	return delegate(ctx, "Daemon.KnownNetworks", d.coreDaemon, func(ctx context.Context, c core.DaemonIface) ([]KnownNetworkRef, error) {
		refs, err := c.KnownNetworks(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]KnownNetworkRef, 0, len(refs))
		for _, r := range refs {
			out = append(out, KnownNetworkRef{Path: r.Path, Name: r.Name})
		}
		return out, nil
	})
}
