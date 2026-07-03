//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

// NOTE: These tests manipulate package-level seam variables (systemConnectFn/sessionConnectFn)
// and therefore MUST NOT run in parallel.

func TestClient(t *testing.T) {
	t.Run("NewClient", func(t *testing.T) {
		type busCase struct {
			name string
			arg  Bus
			set  func(fn func(context.Context) (*connect.Wiring, error))
		}
		buses := []busCase{
			{
				name: "SystemBus",
				arg:  SystemBus,
				set: func(fn func(context.Context) (*connect.Wiring, error)) {
					systemConnectFn = fn
				},
			},
			{
				name: "SessionBus",
				arg:  SessionBus,
				set: func(fn func(context.Context) (*connect.Wiring, error)) {
					sessionConnectFn = fn
				},
			},
		}

		for _, bus := range buses {
			t.Run(bus.name, func(t *testing.T) {
				ctx := context.Background()

				t.Run("Success", func(t *testing.T) {
					fakeDaemon := &fakeCoreDaemon{}
					fakeDaemon.setInfo(&core.DaemonInfo{
						Version:        "1",
						StateDirectory: "/x",
					})
					fakeAdapter := &fakeCoreAdapter{}
					fakeAdapter.powered.Store(true)
					fakeAdapter.name.Store("phy0")
					fakeAdapter.model.Store(new("Broadcomm"))
					fakeAdapter.modes.Store([]core.Mode{core.ModeAP, core.ModeAdHoc})

					resetClientSeams(t)
					bus.set(func(ctx context.Context) (*connect.Wiring, error) {
						return &connect.Wiring{
							Conn:    &dbus.Conn{},
							Daemon:  fakeDaemon,
							Cleanup: func() error { return nil },
						}, nil
					})

					c, err := NewClient(ctx, bus.arg)
					require.NoError(t, err)
					require.NotNil(t, c)

					out, err := c.Daemon().Info(ctx)
					require.NoError(t, err)
					require.Equal(t, "1", out.Version)
				})

				t.Run("ConnectErrorMapsToPublicError", func(t *testing.T) {
					base := errors.New("bus failed")

					resetClientSeams(t)
					bus.set(func(ctx context.Context) (*connect.Wiring, error) {
						return nil, base
					})

					c, err := NewClient(ctx, bus.arg)
					require.Nil(t, c)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrInternal)
					require.ErrorIs(t, err, base)
				})

				t.Run("NilWiring", func(t *testing.T) {
					resetClientSeams(t)
					bus.set(func(ctx context.Context) (*connect.Wiring, error) {
						return nil, nil
					})

					c, err := NewClient(ctx, bus.arg)
					require.Nil(t, c)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrInternal)
				})

				t.Run("WiringMissingDaemon", func(t *testing.T) {
					resetClientSeams(t)
					bus.set(func(ctx context.Context) (*connect.Wiring, error) {
						return &connect.Wiring{
							Conn:    &dbus.Conn{},
							Daemon:  nil,
							Cleanup: func() error { return nil },
						}, nil
					})

					c, err := NewClient(ctx, bus.arg)
					require.Nil(t, c)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrInternal)
				})

				t.Run("ErrorMessageStable", func(t *testing.T) {
					base := errors.New("fail")

					resetClientSeams(t)
					bus.set(func(ctx context.Context) (*connect.Wiring, error) {
						return nil, base
					})

					_, err := NewClient(ctx, bus.arg)
					require.Error(t, err)

					m1 := err.Error()
					m2 := err.Error()
					require.Equal(t, m1, m2)
				})
			})
		}
	})

	t.Run("newClientFromWiring", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			fakeCoreDaemon := &fakeCoreDaemon{}
			fakeCoreDaemon.setInfo(&core.DaemonInfo{
				Version:        "1",
				StateDirectory: "/state",
			})
			fakeCoreAdapter := &fakeCoreAdapter{}
			fakeCoreAdapter.powered.Store(true)
			fakeCoreAdapter.name.Store("phy0")
			fakeCoreAdapter.model.Store(new("Broadcomm"))
			fakeCoreAdapter.modes.Store([]core.Mode{core.ModeAP, core.ModeAdHoc})
			w := &connect.Wiring{
				Conn:    &dbus.Conn{},
				Daemon:  fakeCoreDaemon,
				Cleanup: func() error { return nil },
			}

			c, err := newClientFromWiring(w)
			require.NoError(t, err)
			require.NotNil(t, c)

			out, err := c.Daemon().Info(context.Background())
			require.NoError(t, err)
			require.Equal(t, "1", out.Version)
		})

		t.Run("Nil", func(t *testing.T) {
			c, err := newClientFromWiring(nil)
			require.Nil(t, c)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			cleanupCalled := false
			c := &Client{
				cleanup: func() error {
					cleanupCalled = true
					return nil
				},
			}

			require.NoError(t, c.Close())
			require.True(t, cleanupCalled)
		})

		t.Run("WrappedError", func(t *testing.T) {
			base := errors.New("close failed")
			c := &Client{cleanup: func() error { return base }}

			err := c.Close()
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
			require.ErrorIs(t, err, base)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			var c *Client
			require.NoError(t, c.Close())
		})

		t.Run("NilCleanup", func(t *testing.T) {
			c := &Client{cleanup: nil}
			require.NoError(t, c.Close())
		})

		t.Run("Idempotent", func(t *testing.T) {
			calls := 0
			c := &Client{cleanup: func() error {
				calls++
				return nil
			}}

			require.NoError(t, c.Close())
			require.NoError(t, c.Close())
			require.Equal(t, 1, calls)
		})

		t.Run("ErrorMessageStable", func(t *testing.T) {
			base := errors.New("x")
			c := &Client{cleanup: func() error { return base }}

			err := c.Close()
			require.Error(t, err)

			m1 := err.Error()
			m2 := err.Error()
			require.Equal(t, m1, m2)
		})
	})

	t.Run("Daemon", func(t *testing.T) {
		t.Run("ReturnsDaemonWrapper", func(t *testing.T) {
			fakeCore := &fakeCoreDaemon{}
			fakeCore.setInfo(&core.DaemonInfo{})
			fakeCore.setInfoVersion("1")
			c := &Client{daemon: newDaemon(fakeCore)}

			d := c.Daemon()
			require.NotNil(t, d)

			version, err := d.Version(context.Background())
			require.NoError(t, err)
			require.Equal(t, "1", version)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			var c *Client
			require.Nil(t, c.Daemon())
		})

		t.Run("NoDaemon", func(t *testing.T) {
			c := &Client{daemon: nil}
			require.Nil(t, c.Daemon())
		})
	})
}

func TestClientAllAdapters(t *testing.T) {
	ctx := context.Background()

	// newAllAdaptersClient builds a Client whose daemon enumerates the supplied
	// refs and whose wiring constructs handles via factory. factory may be nil,
	// in which case each path yields a fakeCoreAdapter named after its path.
	newAllAdaptersClient := func(
		refs []core.AdapterRef,
		daemonErr error,
		factory func(ctx context.Context, path string) (core.AdapterIface, error),
	) *Client {
		fakeDaemon := &fakeCoreDaemon{}
		fakeDaemon.setAdapters(refs)
		if daemonErr != nil {
			fakeDaemon.setErr(daemonErr)
		}
		if factory == nil {
			factory = func(_ context.Context, path string) (core.AdapterIface, error) {
				fa := &fakeCoreAdapter{}
				fa.name.Store(path)
				return fa, nil
			}
		}
		wire := &connect.Wiring{
			Conn:           &dbus.Conn{},
			Daemon:         fakeDaemon,
			Cleanup:        func() error { return nil },
			AdapterFactory: factory,
		}
		return &Client{
			daemon:  newDaemon(fakeDaemon),
			wire:    wire,
			cleanup: wire.Cleanup,
		}
	}

	t.Run("Success", func(t *testing.T) {
		refs := []core.AdapterRef{
			{Path: "/net/connman/iwd/phy0", Name: "phy0"},
			{Path: "/net/connman/iwd/phy1", Name: "phy1"},
			{Path: "/net/connman/iwd/phy2", Name: "phy2"},
		}
		c := newAllAdaptersClient(refs, nil, nil)

		adapters, err := c.AllAdapters(ctx)
		require.NoError(t, err)
		require.Len(t, adapters, len(refs))

		// Order is preserved and each handle is live: the fake names each
		// adapter after the path it was constructed from. Path reflects the
		// ref the handle was built from without a backend call.
		for i, a := range adapters {
			require.NotNil(t, a)
			require.Equal(t, refs[i].Path, a.Path())
			name, err := a.Name(ctx)
			require.NoError(t, err)
			require.Equal(t, refs[i].Path, name)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := newAllAdaptersClient(nil, nil, nil)

		adapters, err := c.AllAdapters(ctx)
		require.NoError(t, err)
		require.NotNil(t, adapters)
		require.Empty(t, adapters)
	})

	t.Run("EnumerationErrorMapsToPublicError", func(t *testing.T) {
		base := errors.New("enumeration failed")
		c := newAllAdaptersClient(nil, base, nil)

		adapters, err := c.AllAdapters(ctx)
		require.Nil(t, adapters)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
		require.ErrorIs(t, err, base)
	})

	t.Run("ConstructionErrorFailsFast", func(t *testing.T) {
		refs := []core.AdapterRef{
			{Path: "/net/connman/iwd/phy0", Name: "phy0"},
			{Path: "/net/connman/iwd/phy1", Name: "phy1"},
			{Path: "/net/connman/iwd/phy2", Name: "phy2"},
		}
		base := errors.New("adapter unavailable")
		var constructed []string
		factory := func(_ context.Context, path string) (core.AdapterIface, error) {
			constructed = append(constructed, path)
			if path == refs[1].Path {
				return nil, base
			}
			fa := &fakeCoreAdapter{}
			fa.name.Store(path)
			return fa, nil
		}
		c := newAllAdaptersClient(refs, nil, factory)

		adapters, err := c.AllAdapters(ctx)
		require.Nil(t, adapters)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
		require.ErrorIs(t, err, base)

		// Fail-fast: stopped at the failing adapter, never reached phy2.
		require.Equal(t, []string{refs[0].Path, refs[1].Path}, constructed)
	})

	t.Run("Closed", func(t *testing.T) {
		refs := []core.AdapterRef{{Path: "/net/connman/iwd/phy0", Name: "phy0"}}
		c := newAllAdaptersClient(refs, nil, nil)
		require.NoError(t, c.Close())

		adapters, err := c.AllAdapters(ctx)
		require.Nil(t, adapters)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidState)

		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, KindInvalidState, pe.Kind)
		require.Equal(t, ResourceClient, pe.Resource)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		adapters, err := c.AllAdapters(ctx)
		require.Nil(t, adapters)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("UninitializedWiring", func(t *testing.T) {
		c := &Client{}
		adapters, err := c.AllAdapters(ctx)
		require.Nil(t, adapters)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestClientDevice(t *testing.T) {
	ctx := context.Background()

	newDeviceClient := func(factory func(ctx context.Context, path string) (core.DeviceIface, error)) *Client {
		if factory == nil {
			factory = func(_ context.Context, path string) (core.DeviceIface, error) {
				fd := &fakeCoreDevice{}
				fd.name.Store(path)
				return fd, nil
			}
		}
		wire := &connect.Wiring{
			Conn:          &dbus.Conn{},
			Daemon:        &fakeCoreDaemon{},
			Cleanup:       func() error { return nil },
			DeviceFactory: factory,
		}
		return &Client{daemon: newDaemon(&fakeCoreDaemon{}), wire: wire, cleanup: wire.Cleanup}
	}

	t.Run("Success", func(t *testing.T) {
		c := newDeviceClient(nil)
		d, err := c.Device(ctx, "/net/connman/iwd/phy0/wlan0")
		require.NoError(t, err)
		require.NotNil(t, d)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", d.Path())
	})

	t.Run("WiringErrorMapsToPublicError", func(t *testing.T) {
		base := errors.New("device unavailable")
		c := newDeviceClient(func(_ context.Context, _ string) (core.DeviceIface, error) {
			return nil, base
		})
		d, err := c.Device(ctx, "/net/connman/iwd/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.ErrorIs(t, err, base)
	})

	t.Run("Closed", func(t *testing.T) {
		c := newDeviceClient(nil)
		require.NoError(t, c.Close())

		d, err := c.Device(ctx, "/net/connman/iwd/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidState)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		d, err := c.Device(ctx, "/net/connman/iwd/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("UninitializedWiring", func(t *testing.T) {
		c := &Client{}
		d, err := c.Device(ctx, "/net/connman/iwd/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestClientAllDevices(t *testing.T) {
	ctx := context.Background()

	// newAllDevicesClient builds a Client whose daemon enumerates the supplied
	// refs and whose wiring constructs handles via factory. factory may be nil,
	// in which case each path yields a fakeCoreDevice named after its path.
	newAllDevicesClient := func(
		refs []core.DeviceRef,
		daemonErr error,
		factory func(ctx context.Context, path string) (core.DeviceIface, error),
	) *Client {
		fakeDaemon := &fakeCoreDaemon{}
		fakeDaemon.setDevices(refs)
		if daemonErr != nil {
			fakeDaemon.setErr(daemonErr)
		}
		if factory == nil {
			factory = func(_ context.Context, path string) (core.DeviceIface, error) {
				fd := &fakeCoreDevice{}
				fd.name.Store(path)
				return fd, nil
			}
		}
		wire := &connect.Wiring{
			Conn:          &dbus.Conn{},
			Daemon:        fakeDaemon,
			Cleanup:       func() error { return nil },
			DeviceFactory: factory,
		}
		return &Client{
			daemon:  newDaemon(fakeDaemon),
			wire:    wire,
			cleanup: wire.Cleanup,
		}
	}

	t.Run("Success", func(t *testing.T) {
		refs := []core.DeviceRef{
			{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"},
			{Path: "/net/connman/iwd/phy1/wlan1", Name: "wlan1"},
		}
		c := newAllDevicesClient(refs, nil, nil)

		devices, err := c.AllDevices(ctx)
		require.NoError(t, err)
		require.Len(t, devices, len(refs))

		// Order is preserved and each handle is live: the fake names each
		// device after the path it was constructed from.
		for i, d := range devices {
			require.NotNil(t, d)
			require.Equal(t, refs[i].Path, d.Path())
			name, err := d.Name(ctx)
			require.NoError(t, err)
			require.Equal(t, refs[i].Path, name)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := newAllDevicesClient(nil, nil, nil)

		devices, err := c.AllDevices(ctx)
		require.NoError(t, err)
		require.NotNil(t, devices)
		require.Empty(t, devices)
	})

	t.Run("EnumerationErrorMapsToPublicError", func(t *testing.T) {
		base := errors.New("enumeration failed")
		c := newAllDevicesClient(nil, base, nil)

		devices, err := c.AllDevices(ctx)
		require.Nil(t, devices)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
		require.ErrorIs(t, err, base)
	})

	t.Run("ConstructionErrorFailsFast", func(t *testing.T) {
		refs := []core.DeviceRef{
			{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"},
			{Path: "/net/connman/iwd/phy1/wlan1", Name: "wlan1"},
			{Path: "/net/connman/iwd/phy2/wlan2", Name: "wlan2"},
		}
		base := errors.New("device unavailable")
		var constructed []string
		factory := func(_ context.Context, path string) (core.DeviceIface, error) {
			constructed = append(constructed, path)
			if path == refs[1].Path {
				return nil, base
			}
			fd := &fakeCoreDevice{}
			fd.name.Store(path)
			return fd, nil
		}
		c := newAllDevicesClient(refs, nil, factory)

		devices, err := c.AllDevices(ctx)
		require.Nil(t, devices)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
		require.ErrorIs(t, err, base)

		// Fail-fast: stopped at the failing device, never reached wlan2.
		require.Equal(t, []string{refs[0].Path, refs[1].Path}, constructed)
	})

	t.Run("Closed", func(t *testing.T) {
		refs := []core.DeviceRef{{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"}}
		c := newAllDevicesClient(refs, nil, nil)
		require.NoError(t, c.Close())

		devices, err := c.AllDevices(ctx)
		require.Nil(t, devices)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidState)

		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, KindInvalidState, pe.Kind)
		require.Equal(t, ResourceClient, pe.Resource)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		devices, err := c.AllDevices(ctx)
		require.Nil(t, devices)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("UninitializedWiring", func(t *testing.T) {
		c := &Client{}
		devices, err := c.AllDevices(ctx)
		require.Nil(t, devices)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func resetClientSeams(t *testing.T) {
	t.Helper()

	systemOrig := systemConnectFn
	sessionOrig := sessionConnectFn
	t.Cleanup(func() {
		systemConnectFn = systemOrig
		sessionConnectFn = sessionOrig
	})
}
