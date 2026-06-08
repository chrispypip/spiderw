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

// DaemonIface defines the core daemon operations used by the public layer.
type DaemonIface interface {
	Info(ctx context.Context) (*DaemonInfo, error)
	Version(ctx context.Context) (string, error)
	StateDirectory(ctx context.Context) (string, error)
	NetworkConfigurationEnabled(ctx context.Context) (bool, error)
	Adapters(ctx context.Context) ([]AdapterRef, error)
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
