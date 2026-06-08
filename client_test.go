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
					mockModel := "Broadcomm"
					fakeAdapter := &fakeCoreAdapter{}
					fakeAdapter.powered.Store(true)
					fakeAdapter.name.Store("phy0")
					fakeAdapter.model.Store(&mockModel)
					fakeAdapter.modes.Store([]core.AdapterMode{core.AdapterModeAP, core.AdapterModeAdHoc})

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
			mockModel := "Broadcomm"
			fakeCoreAdapter := &fakeCoreAdapter{}
			fakeCoreAdapter.powered.Store(true)
			fakeCoreAdapter.name.Store("phy0")
			fakeCoreAdapter.model.Store(&mockModel)
			fakeCoreAdapter.modes.Store([]core.AdapterMode{core.AdapterModeAP, core.AdapterModeAdHoc})
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

func resetClientSeams(t *testing.T) {
	t.Helper()

	systemOrig := systemConnectFn
	sessionOrig := sessionConnectFn
	t.Cleanup(func() {
		systemConnectFn = systemOrig
		sessionConnectFn = sessionOrig
	})
}
