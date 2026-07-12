package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// AdapterRef is a lightweight reference to an adapter discovered by the iwd daemon.
type AdapterRef struct {
	// Path is the canonical D-Bus object path for the adapter.
	Path string

	// Name is the adapter's Name property.
	Name string
}

// DeviceRef is a lightweight reference to a device discovered by the iwd daemon.
type DeviceRef struct {
	// Path is the canonical D-Bus object path for the device.
	Path string

	// Name is the device's Name property.
	Name string
}

// BasicServiceSetRef is a lightweight reference to a basic service set (BSS)
// discovered by the iwd daemon.
type BasicServiceSetRef struct {
	// Path is the canonical D-Bus object path for the BSS.
	Path string

	// Address is the BSS's Address (BSSID) property.
	Address string
}

// NetworkRef is a lightweight reference to a network discovered by the iwd
// daemon.
type NetworkRef struct {
	// Path is the canonical D-Bus object path for the network.
	Path string

	// Name is the network's Name (SSID) property.
	Name string
}

// KnownNetworkRef is a lightweight reference to a known network discovered by the
// iwd daemon.
type KnownNetworkRef struct {
	// Path is the canonical D-Bus object path for the known network.
	Path string

	// Name is the known network's Name property.
	Name string
}

// StationRef is a lightweight reference to a station discovered by the iwd
// daemon. A station has no Name of its own, so it is identified by its path (the
// owning device's path).
type StationRef struct {
	// Path is the canonical D-Bus object path for the station.
	Path string

	// Name is the co-located device's Name (e.g. "wlan0"); best-effort, may be
	// empty. A station has no Name of its own.
	Name string
}

// AccessPointRef is a lightweight reference to an access point (a device in AP
// mode) discovered by the iwd daemon. Name is the co-located device's Name (e.g.
// "wlan1"), not the hosted network's SSID.
type AccessPointRef struct {
	// Path is the canonical D-Bus object path for the access point.
	Path string

	// Name is the co-located device's Name (best-effort, may be empty).
	Name string
}

// DaemonIface defines the core daemon operations used by the public layer.
type DaemonIface interface {
	Info(ctx context.Context) (*DaemonInfo, error)
	Version(ctx context.Context) (string, error)
	StateDirectory(ctx context.Context) (string, error)
	NetworkConfigurationEnabled(ctx context.Context) (bool, error)
	Adapters(ctx context.Context) ([]AdapterRef, error)
	Devices(ctx context.Context) ([]DeviceRef, error)
	Stations(ctx context.Context) ([]StationRef, error)
	AccessPoints(ctx context.Context) ([]AccessPointRef, error)
	BasicServiceSets(ctx context.Context) ([]BasicServiceSetRef, error)
	Networks(ctx context.Context) ([]NetworkRef, error)
	KnownNetworks(ctx context.Context) ([]KnownNetworkRef, error)
}

// DaemonInfo is the normalized core-layer view of daemon metadata.
type DaemonInfo struct {
	// Version is the normalized daemon version string.
	Version string

	// StateDirectory is the normalized daemon state directory path.
	StateDirectory string

	// NetworkConfigurationEnabled reports whether iwd manages network configuration.
	NetworkConfigurationEnabled bool
}

type daemonRaw interface {
	GetInfo(ctx context.Context) (*iwdbus.DaemonInfo, error)
	GetAdapters(ctx context.Context) ([]iwdbus.AdapterRef, error)
	GetDevices(ctx context.Context) ([]iwdbus.DeviceRef, error)
	GetStations(ctx context.Context) ([]iwdbus.StationRef, error)
	GetAccessPoints(ctx context.Context) ([]iwdbus.AccessPointRef, error)
	GetBasicServiceSets(ctx context.Context) ([]iwdbus.BasicServiceSetRef, error)
	GetNetworks(ctx context.Context) ([]iwdbus.NetworkRef, error)
	GetKnownNetworks(ctx context.Context) ([]iwdbus.KnownNetworkRef, error)
}

// Daemon is the core-layer facade over a raw iwd daemon backend.
type Daemon struct {
	raw daemonRaw
}

// NewDaemon wraps a raw daemon backend in a core-layer Daemon.
func NewDaemon(raw daemonRaw) *Daemon {
	if raw == nil {
		return nil
	}
	return &Daemon{raw: raw}
}

func (d *Daemon) rawDaemon(op string) (daemonRaw, error) {
	if d == nil || d.raw == nil {
		return nil, WrapInvalidState(ResourceDaemon, op, "daemon wrapper was nil", ErrDaemonNotInitialized)
	}
	return d.raw, nil
}

// Info returns normalized daemon metadata.
func (d *Daemon) Info(ctx context.Context) (*DaemonInfo, error) {
	const op = "Daemon.Info"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawInfo, err := rawDaemon.GetInfo(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed querying iwd Daemon info", err)
	}

	v := strings.TrimSpace(rawInfo.Version)
	if v == "" {
		err := WrapInvalidState(ResourceDaemon, op, "daemon returned empty Version", fmt.Errorf("missing or invalid Version field"))
		return nil, err
	}

	s := strings.TrimSpace(rawInfo.StateDirectory)
	if s == "" {
		err := WrapInvalidState(ResourceDaemon, op, "daemon returned empty StateDirectory", fmt.Errorf("missing or invalid StateDirectory field"))
		return nil, err
	}
	if !strings.HasPrefix(s, "/") {
		err := WrapInvalidState(ResourceDaemon, op, "daemon returned StateDirectory as relative path", fmt.Errorf("invalid StateDirectory field"))
		return nil, err
	}

	info := &DaemonInfo{
		Version:                     v,
		StateDirectory:              s,
		NetworkConfigurationEnabled: rawInfo.NetworkConfigurationEnabled,
	}
	return info, nil
}

// Version returns the normalized daemon version.
func (d *Daemon) Version(ctx context.Context) (string, error) {
	info, err := d.Info(ctx)
	if err != nil {
		return "", err
	}
	return info.Version, nil
}

// StateDirectory returns the normalized daemon state directory.
func (d *Daemon) StateDirectory(ctx context.Context) (string, error) {
	info, err := d.Info(ctx)
	if err != nil {
		return "", err
	}
	return info.StateDirectory, nil
}

// NetworkConfigurationEnabled reports whether iwd manages network configuration.
func (d *Daemon) NetworkConfigurationEnabled(ctx context.Context) (bool, error) {
	info, err := d.Info(ctx)
	if err != nil {
		return false, err
	}
	return info.NetworkConfigurationEnabled, nil
}

// Adapters returns the adapters currently exposed by the iwd daemon.
func (d *Daemon) Adapters(ctx context.Context) ([]AdapterRef, error) {
	const op = "Daemon.Adapters"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetAdapters(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting adapters", err)
	}
	refs := make([]AdapterRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceAdapter, op, "adapter returned invalid path", fmt.Errorf("invalid adapter path %q", p))
		}

		n := strings.TrimSpace(r.Name)
		if n == "" {
			return nil, WrapInvalidState(ResourceAdapter, op, "adapter returned empty Name", fmt.Errorf("missing or invalid Name field"))
		}

		refs = append(refs, AdapterRef{Path: p, Name: n})
	}
	return refs, nil
}

// Devices returns the devices currently exposed by the iwd daemon.
func (d *Daemon) Devices(ctx context.Context) ([]DeviceRef, error) {
	const op = "Daemon.Devices"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetDevices(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting devices", err)
	}
	refs := make([]DeviceRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceDevice, op, "device returned invalid path", fmt.Errorf("invalid device path %q", p))
		}

		n := strings.TrimSpace(r.Name)
		if n == "" {
			return nil, WrapInvalidState(ResourceDevice, op, "device returned empty Name", fmt.Errorf("missing or invalid Name field"))
		}

		refs = append(refs, DeviceRef{Path: p, Name: n})
	}
	return refs, nil
}

// Stations returns the stations (station-mode devices) currently exposed by the
// iwd daemon. A station has no Name, so each ref carries only its path.
func (d *Daemon) Stations(ctx context.Context) ([]StationRef, error) {
	const op = "Daemon.Stations"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetStations(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting stations", err)
	}
	refs := make([]StationRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceStation, op, "station returned invalid path", fmt.Errorf("invalid station path %q", p))
		}
		refs = append(refs, StationRef{Path: p, Name: strings.TrimSpace(r.Name)})
	}
	return refs, nil
}

// AccessPoints returns the access points (devices in AP mode) currently exposed
// by the iwd daemon. Each ref carries the co-located device Name.
func (d *Daemon) AccessPoints(ctx context.Context) ([]AccessPointRef, error) {
	const op = "Daemon.AccessPoints"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetAccessPoints(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting access points", err)
	}
	refs := make([]AccessPointRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceAccessPoint, op, "access point returned invalid path", fmt.Errorf("invalid access point path %q", p))
		}
		refs = append(refs, AccessPointRef{Path: p, Name: strings.TrimSpace(r.Name)})
	}
	return refs, nil
}

// BasicServiceSets returns the basic service sets currently exposed by the iwd
// daemon.
func (d *Daemon) BasicServiceSets(ctx context.Context) ([]BasicServiceSetRef, error) {
	const op = "Daemon.BasicServiceSets"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetBasicServiceSets(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting basic service sets", err)
	}
	refs := make([]BasicServiceSetRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceBasicServiceSet, op, "basic service set returned invalid path", fmt.Errorf("invalid basic service set path %q", p))
		}

		a := strings.TrimSpace(r.Address)
		if a == "" {
			return nil, WrapInvalidState(ResourceBasicServiceSet, op, "basic service set returned empty Address", fmt.Errorf("missing or invalid Address field"))
		}

		refs = append(refs, BasicServiceSetRef{Path: p, Address: a})
	}
	return refs, nil
}

// Networks returns the networks currently exposed by the iwd daemon.
func (d *Daemon) Networks(ctx context.Context) ([]NetworkRef, error) {
	const op = "Daemon.Networks"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetNetworks(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting networks", err)
	}
	refs := make([]NetworkRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceNetwork, op, "network returned invalid path", fmt.Errorf("invalid network path %q", p))
		}

		n := strings.TrimSpace(r.Name)
		if n == "" {
			return nil, WrapInvalidState(ResourceNetwork, op, "network returned empty Name", fmt.Errorf("missing or invalid Name field"))
		}

		refs = append(refs, NetworkRef{Path: p, Name: n})
	}
	return refs, nil
}

// KnownNetworks returns the known networks currently exposed by the iwd daemon.
func (d *Daemon) KnownNetworks(ctx context.Context) ([]KnownNetworkRef, error) {
	const op = "Daemon.KnownNetworks"

	rawDaemon, err := d.rawDaemon(op)
	if err != nil {
		return nil, err
	}

	rawRefs, err := rawDaemon.GetKnownNetworks(ctx)
	if err != nil {
		return nil, WrapDaemonUnavailable(op, "failed getting known networks", err)
	}
	refs := make([]KnownNetworkRef, 0, len(rawRefs))
	for _, r := range rawRefs {
		p := strings.TrimSpace(string(r.Path))
		if p == "" || !strings.HasPrefix(p, "/") {
			return nil, WrapInvalidState(ResourceKnownNetwork, op, "known network returned invalid path", fmt.Errorf("invalid known network path %q", p))
		}

		n := strings.TrimSpace(r.Name)
		if n == "" {
			return nil, WrapInvalidState(ResourceKnownNetwork, op, "known network returned empty Name", fmt.Errorf("missing or invalid Name field"))
		}

		refs = append(refs, KnownNetworkRef{Path: p, Name: n})
	}
	return refs, nil
}
