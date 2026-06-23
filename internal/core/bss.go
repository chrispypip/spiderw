package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

type bssRaw interface {
	GetAddress(ctx context.Context) (string, error)
	GetProperties(ctx context.Context) (*iwdbus.BasicServiceSetProperties, error)
}

// BasicServiceSetIface defines the core BSS operations used by the public layer.
type BasicServiceSetIface interface {
	Address(ctx context.Context) (string, error)
	Properties(ctx context.Context) (*BasicServiceSetProperties, error)
}

// BasicServiceSetProperties holds normalized BSS properties read in a single
// backend call. The iwd BasicServiceSet interface exposes only Address.
type BasicServiceSetProperties struct {
	Address string
}

// BasicServiceSet is the core-layer facade over a raw iwd BSS backend.
type BasicServiceSet struct {
	raw bssRaw
}

// NewBasicServiceSet wraps a raw BSS backend in a core-layer BasicServiceSet.
func NewBasicServiceSet(raw bssRaw) *BasicServiceSet {
	if raw == nil {
		return nil
	}
	return &BasicServiceSet{raw: raw}
}

func (b *BasicServiceSet) rawBSS(op string) (bssRaw, error) {
	if b == nil || b.raw == nil {
		return nil, WrapInvalidState(ResourceBasicServiceSet, op, "basic service set wrapper was nil", ErrBasicServiceSetNotInitialized)
	}
	return b.raw, nil
}

// Address returns the normalized BSS hardware (BSSID) address.
func (b *BasicServiceSet) Address(ctx context.Context) (string, error) {
	const op = "BasicServiceSet.Address"

	rawBSS, err := b.rawBSS(op)
	if err != nil {
		return "", err
	}

	raw, err := rawBSS.GetAddress(ctx)
	if err != nil {
		return "", WrapBasicServiceSetUnavailable(op, "failed querying iwd BasicServiceSet address", err)
	}

	a := strings.TrimSpace(raw)
	if a == "" {
		return "", WrapInvalidState(ResourceBasicServiceSet, op, "basic service set returned empty Address", fmt.Errorf("missing or invalid Address field"))
	}

	return a, nil
}

// Properties returns all normalized BSS properties read in a single backend call
// (Properties.GetAll), applying the same normalization as the per-property
// getter: Address is trimmed and required.
func (b *BasicServiceSet) Properties(ctx context.Context) (*BasicServiceSetProperties, error) {
	const op = "BasicServiceSet.Properties"

	rawBSS, err := b.rawBSS(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawBSS.GetProperties(ctx)
	if err != nil {
		return nil, WrapBasicServiceSetUnavailable(op, "failed querying iwd BasicServiceSet properties", err)
	}

	address := strings.TrimSpace(raw.Address)
	if address == "" {
		return nil, WrapInvalidState(ResourceBasicServiceSet, op, "basic service set returned empty Address", fmt.Errorf("missing or invalid Address field"))
	}

	return &BasicServiceSetProperties{Address: address}, nil
}
