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

// TestClient_ObjectLookups covers the single-object lookup entry points
// (Client.Adapter/Network/KnownNetwork/BasicServiceSet), which construct a
// handle from a path via the shared clientObject helper.
func TestClient_ObjectLookups(t *testing.T) {
	ctx := context.Background()

	newClient := func(wire *connect.Wiring) *Client {
		return &Client{daemon: newDaemon(&fakeCoreDaemon{}), wire: wire, cleanup: wire.Cleanup}
	}

	for _, tc := range []struct {
		name     string
		wire     func(fail bool) *connect.Wiring
		lookup   func(c *Client) (string, error) // returns handle path
		wantPath string
	}{
		{
			name: "Adapter",
			wire: func(fail bool) *connect.Wiring {
				return &connect.Wiring{Conn: &dbus.Conn{}, Cleanup: func() error { return nil },
					AdapterFactory: func(ctx context.Context, path string) (core.AdapterIface, error) {
						if fail {
							return nil, errors.New("wire boom")
						}
						return &fakeCoreAdapter{}, nil
					}}
			},
			lookup:   func(c *Client) (string, error) { a, err := c.Adapter(ctx, "/adapter/p"); return a.Path(), err },
			wantPath: "/adapter/p",
		},
		{
			name: "BasicServiceSet",
			wire: func(fail bool) *connect.Wiring {
				return &connect.Wiring{Conn: &dbus.Conn{}, Cleanup: func() error { return nil },
					BasicServiceSetFactory: func(ctx context.Context, path string) (core.BasicServiceSetIface, error) {
						if fail {
							return nil, errors.New("wire boom")
						}
						return &fakeCoreBSS{}, nil
					}}
			},
			lookup:   func(c *Client) (string, error) { b, err := c.BasicServiceSet(ctx, "/bss/p"); return b.Path(), err },
			wantPath: "/bss/p",
		},
		{
			name: "Network",
			wire: func(fail bool) *connect.Wiring {
				return &connect.Wiring{Conn: &dbus.Conn{}, ResolverOverride: connect.NoResolver{}, Cleanup: func() error { return nil },
					NetworkFactory: func(ctx context.Context, path string) (core.NetworkIface, error) {
						if fail {
							return nil, errors.New("wire boom")
						}
						return &fakeCoreNetwork{}, nil
					}}
			},
			lookup:   func(c *Client) (string, error) { n, err := c.Network(ctx, "/net/p"); return n.Path(), err },
			wantPath: "/net/p",
		},
		{
			name: "KnownNetwork",
			wire: func(fail bool) *connect.Wiring {
				return &connect.Wiring{Conn: &dbus.Conn{}, ResolverOverride: connect.NoResolver{}, Cleanup: func() error { return nil },
					KnownNetworkFactory: func(ctx context.Context, path string) (core.KnownNetworkIface, error) {
						if fail {
							return nil, errors.New("wire boom")
						}
						return &fakeCoreKnownNetwork{}, nil
					}}
			},
			lookup:   func(c *Client) (string, error) { k, err := c.KnownNetwork(ctx, "/known/p"); return k.Path(), err },
			wantPath: "/known/p",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				path, err := tc.lookup(newClient(tc.wire(false)))
				require.NoError(t, err)
				require.Equal(t, tc.wantPath, path)
			})

			t.Run("WiringError", func(t *testing.T) {
				_, err := tc.lookup(newClient(tc.wire(true)))
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})

			t.Run("Closed", func(t *testing.T) {
				c := newClient(tc.wire(false))
				require.NoError(t, c.Close())
				_, err := tc.lookup(c)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidState)
			})

			t.Run("NilReceiver", func(t *testing.T) {
				_, err := tc.lookup(nil)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})
		})
	}
}

func TestClient_ObjectWrapperNil(t *testing.T) {
	ctx := context.Background()
	// A factory that yields a nil core object makes the wrapper nil, which the
	// shared clientObject helper reports as an internal error.
	wire := &connect.Wiring{Conn: &dbus.Conn{}, Cleanup: func() error { return nil },
		AdapterFactory: func(ctx context.Context, path string) (core.AdapterIface, error) {
			return nil, nil
		}}
	c := &Client{daemon: newDaemon(&fakeCoreDaemon{}), wire: wire, cleanup: wire.Cleanup}

	_, err := c.Adapter(ctx, "/p")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInternal)
}

func TestValidateClientWiring(t *testing.T) {
	conn := &dbus.Conn{}
	daemon := &fakeCoreDaemon{}
	cleanup := func() error { return nil }

	for _, tc := range []struct {
		name    string
		wire    *connect.Wiring
		wantErr bool
	}{
		{"NilWiring", nil, true},
		{"NilConn", &connect.Wiring{Daemon: daemon, Cleanup: cleanup}, true},
		{"NilDaemon", &connect.Wiring{Conn: conn, Cleanup: cleanup}, true},
		{"NilCleanup", &connect.Wiring{Conn: conn, Daemon: daemon}, true},
		{"Valid", &connect.Wiring{Conn: conn, Daemon: daemon, Cleanup: cleanup}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClientWiring("op", tc.wire)
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClient_CloseAndDaemonEdges(t *testing.T) {
	t.Run("CloseWrapsPlainCleanupError", func(t *testing.T) {
		c := &Client{cleanup: func() error { return errors.New("cleanup boom") }}
		err := c.Close()
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, "Client.Close", pe.Op)
	})

	t.Run("ClosePreservesPublicCleanupError", func(t *testing.T) {
		sentinel := &Error{Kind: KindUnavailable, Resource: ResourceClient, Op: "cleanup", Err: ErrUnavailable}
		c := &Client{cleanup: func() error { return sentinel }}
		err := c.Close()
		require.Same(t, sentinel, err)
	})

	t.Run("DaemonNilAfterClose", func(t *testing.T) {
		c := &Client{daemon: newDaemon(&fakeCoreDaemon{}), cleanup: func() error { return nil }}
		require.NotNil(t, c.Daemon())
		require.NoError(t, c.Close())
		require.Nil(t, c.Daemon())
	})
}

func TestClient_RegisterAgentEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("NilReceiver", func(t *testing.T) {
		_, err := (*Client)(nil).RegisterAgent(ctx, validAgentConfig())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("WireNil", func(t *testing.T) {
		c := &Client{daemon: newDaemon(&fakeCoreDaemon{})}
		_, err := c.RegisterAgent(ctx, validAgentConfig())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("WrapperNilWhenFactoryYieldsNilCore", func(t *testing.T) {
		c := newAgentTestClient(t, factoryReturning(nil))
		defer func() { _ = c.Close() }()
		_, err := c.RegisterAgent(ctx, validAgentConfig())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestClient_ResolveStationName(t *testing.T) {
	ctx := context.Background()

	t.Run("NilClientOrDaemon", func(t *testing.T) {
		require.Empty(t, (*Client)(nil).resolveStationName(ctx, "/p"))
		require.Empty(t, (&Client{}).resolveStationName(ctx, "/p"))
	})

	t.Run("EnumerationErrorYieldsEmpty", func(t *testing.T) {
		fd := &fakeCoreDaemon{}
		fd.setErr(errors.New("boom"))
		c := &Client{daemon: newDaemon(fd)}
		require.Empty(t, c.resolveStationName(ctx, "/p"))
	})

	t.Run("MatchAndMiss", func(t *testing.T) {
		fd := &fakeCoreDaemon{}
		fd.setStations([]core.StationRef{{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"}})
		c := &Client{daemon: newDaemon(fd)}
		require.Equal(t, "wlan0", c.resolveStationName(ctx, "/net/connman/iwd/phy0/wlan0"))
		require.Empty(t, c.resolveStationName(ctx, "/net/connman/iwd/phy0/other"))
	})
}

// TestClient_EnumeratorErrorPaths drives the error branches shared by every
// Client.AllX enumerator: a nil wiring, a daemon enumeration failure, a
// per-object wiring failure, and a nil constructed wrapper.
func TestClient_EnumeratorErrorPaths(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")

	for _, e := range []struct {
		name       string
		makeClient func(daemonErr, factoryErr error, nilCore bool) *Client
		call       func(*Client) error
	}{
		{
			name: "AllAdapters",
			makeClient: func(daemonErr, factoryErr error, nilCore bool) *Client {
				fd := &fakeCoreDaemon{}
				fd.setAdapters([]core.AdapterRef{{Path: "/a"}})
				if daemonErr != nil {
					fd.setErr(daemonErr)
				}
				wire := &connect.Wiring{Conn: &dbus.Conn{}, Daemon: fd, Cleanup: func() error { return nil },
					AdapterFactory: func(ctx context.Context, path string) (core.AdapterIface, error) {
						if factoryErr != nil {
							return nil, factoryErr
						}
						if nilCore {
							return nil, nil
						}
						return &fakeCoreAdapter{}, nil
					}}
				return &Client{daemon: newDaemon(fd), wire: wire, cleanup: wire.Cleanup}
			},
			call: func(c *Client) error { _, err := c.AllAdapters(ctx); return err },
		},
		{
			name: "AllDevices",
			makeClient: func(daemonErr, factoryErr error, nilCore bool) *Client {
				fd := &fakeCoreDaemon{}
				fd.setDevices([]core.DeviceRef{{Path: "/d"}})
				if daemonErr != nil {
					fd.setErr(daemonErr)
				}
				wire := &connect.Wiring{Conn: &dbus.Conn{}, Daemon: fd, Cleanup: func() error { return nil },
					DeviceFactory: func(ctx context.Context, path string) (core.DeviceIface, error) {
						if factoryErr != nil {
							return nil, factoryErr
						}
						if nilCore {
							return nil, nil
						}
						return &fakeCoreDevice{}, nil
					}}
				return &Client{daemon: newDaemon(fd), wire: wire, cleanup: wire.Cleanup}
			},
			call: func(c *Client) error { _, err := c.AllDevices(ctx); return err },
		},
		{
			name: "AllStations",
			makeClient: func(daemonErr, factoryErr error, nilCore bool) *Client {
				fd := &fakeCoreDaemon{}
				fd.setStations([]core.StationRef{{Path: "/s"}})
				if daemonErr != nil {
					fd.setErr(daemonErr)
				}
				wire := &connect.Wiring{Conn: &dbus.Conn{}, Daemon: fd, Cleanup: func() error { return nil },
					StationFactory: func(ctx context.Context, path string) (core.StationIface, error) {
						if factoryErr != nil {
							return nil, factoryErr
						}
						if nilCore {
							return nil, nil
						}
						return &fakeCoreStation{}, nil
					}}
				return &Client{daemon: newDaemon(fd), wire: wire, cleanup: wire.Cleanup}
			},
			call: func(c *Client) error { _, err := c.AllStations(ctx); return err },
		},
		{
			name: "AllBasicServiceSets",
			makeClient: func(daemonErr, factoryErr error, nilCore bool) *Client {
				fd := &fakeCoreDaemon{}
				fd.setBasicServiceSets([]core.BasicServiceSetRef{{Path: "/b"}})
				if daemonErr != nil {
					fd.setErr(daemonErr)
				}
				wire := &connect.Wiring{Conn: &dbus.Conn{}, Daemon: fd, Cleanup: func() error { return nil },
					BasicServiceSetFactory: func(ctx context.Context, path string) (core.BasicServiceSetIface, error) {
						if factoryErr != nil {
							return nil, factoryErr
						}
						if nilCore {
							return nil, nil
						}
						return &fakeCoreBSS{}, nil
					}}
				return &Client{daemon: newDaemon(fd), wire: wire, cleanup: wire.Cleanup}
			},
			call: func(c *Client) error { _, err := c.AllBasicServiceSets(ctx); return err },
		},
		{
			name: "AllNetworks",
			makeClient: func(daemonErr, factoryErr error, nilCore bool) *Client {
				fd := &fakeCoreDaemon{}
				fd.setNetworks([]core.NetworkRef{{Path: "/n"}})
				if daemonErr != nil {
					fd.setErr(daemonErr)
				}
				wire := &connect.Wiring{Conn: &dbus.Conn{}, Daemon: fd, Cleanup: func() error { return nil },
					NetworkFactory: func(ctx context.Context, path string) (core.NetworkIface, error) {
						if factoryErr != nil {
							return nil, factoryErr
						}
						if nilCore {
							return nil, nil
						}
						return &fakeCoreNetwork{}, nil
					}}
				return &Client{daemon: newDaemon(fd), wire: wire, cleanup: wire.Cleanup}
			},
			call: func(c *Client) error { _, err := c.AllNetworks(ctx); return err },
		},
		{
			name: "AllKnownNetworks",
			makeClient: func(daemonErr, factoryErr error, nilCore bool) *Client {
				fd := &fakeCoreDaemon{}
				fd.setKnownNetworks([]core.KnownNetworkRef{{Path: "/k"}})
				if daemonErr != nil {
					fd.setErr(daemonErr)
				}
				wire := &connect.Wiring{Conn: &dbus.Conn{}, Daemon: fd, Cleanup: func() error { return nil },
					KnownNetworkFactory: func(ctx context.Context, path string) (core.KnownNetworkIface, error) {
						if factoryErr != nil {
							return nil, factoryErr
						}
						if nilCore {
							return nil, nil
						}
						return &fakeCoreKnownNetwork{}, nil
					}}
				return &Client{daemon: newDaemon(fd), wire: wire, cleanup: wire.Cleanup}
			},
			call: func(c *Client) error { _, err := c.AllKnownNetworks(ctx); return err },
		},
	} {
		t.Run(e.name, func(t *testing.T) {
			t.Run("NilReceiver", func(t *testing.T) {
				err := e.call(nil)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})

			t.Run("Closed", func(t *testing.T) {
				c := e.makeClient(nil, nil, false)
				require.NoError(t, c.Close())
				err := e.call(c)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidState)
			})

			t.Run("WireNil", func(t *testing.T) {
				c := &Client{daemon: newDaemon(&fakeCoreDaemon{})}
				err := e.call(c)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})

			t.Run("EnumerationError", func(t *testing.T) {
				err := e.call(e.makeClient(boom, nil, false))
				require.Error(t, err)
			})

			t.Run("WiringError", func(t *testing.T) {
				err := e.call(e.makeClient(nil, boom, false))
				require.Error(t, err)
			})

			t.Run("WrapperNil", func(t *testing.T) {
				err := e.call(e.makeClient(nil, nil, true))
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})
		})
	}
}

func TestClient(t *testing.T) {
	t.Run("NewClient", func(t *testing.T) {
		type busCase struct {
			name string
			arg  Bus
			set  func(fn func(ctx context.Context) (*connect.Wiring, error))
		}
		buses := []busCase{
			{
				name: "SystemBus",
				arg:  SystemBus,
				set: func(fn func(ctx context.Context) (*connect.Wiring, error)) {
					systemConnectFn = fn
				},
			},
			{
				name: "SessionBus",
				arg:  SessionBus,
				set: func(fn func(ctx context.Context) (*connect.Wiring, error)) {
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

func TestClient_AllAdapters(t *testing.T) {
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
			factory = func(ctx context.Context, path string) (core.AdapterIface, error) {
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
		factory := func(ctx context.Context, path string) (core.AdapterIface, error) {
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
			factory = func(ctx context.Context, path string) (core.DeviceIface, error) {
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
		c := newDeviceClient(func(ctx context.Context, path string) (core.DeviceIface, error) {
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

func TestClient_AllDevices(t *testing.T) {
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
			factory = func(ctx context.Context, path string) (core.DeviceIface, error) {
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
		factory := func(ctx context.Context, path string) (core.DeviceIface, error) {
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
