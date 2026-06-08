//go:build unit

package core

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// NOTE: This file uses a "subset" subtest structure (t.Run trees) so related
// cases are grouped under a small number of top-level tests. This improves
// readability and makes it easy to run targeted slices (e.g. -run TestDaemon/Info).

func TestDaemon_Core(t *testing.T) {
	t.Run("NewDaemon", func(t *testing.T) {
		tests := []struct {
			name    string
			in      daemonRaw
			wantNil bool
		}{
			{name: "nil", in: nil, wantNil: true},
			{name: "non-nil", in: fakeIwdbusDaemonWithInfo(&iwdbus.DaemonInfo{Version: "1", StateDirectory: "/x"}), wantNil: false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				d := NewDaemon(tc.in)
				if tc.wantNil {
					require.Nil(t, d)
					return
				}
				require.NotNil(t, d)
			})
		}
	})

	t.Run("Info", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			tests := []struct {
				name   string
				daemon *Daemon
			}{
				{name: "nil receiver", daemon: nil},
				{name: "inner nil", daemon: &Daemon{raw: nil}},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					_, err := tc.daemon.Info(context.Background())
					require.Error(t, err)
					require.True(t, errors.Is(err, ErrDaemonNotInitialized))
					require.True(t, errors.Is(err, ErrCore))
				})
			}
		})

		t.Run("DBusErrorMapping", func(t *testing.T) {
			tests := []struct {
				name     string
				dbusErr  error
				wantKind Kind
			}{
				{name: "connection", dbusErr: iwdbus.ErrDBusConnection, wantKind: KindUnavailable},
				{name: "method", dbusErr: iwdbus.ErrDBusMethod, wantKind: KindUnavailable},
				{name: "introspection", dbusErr: iwdbus.ErrDBusIntrospection, wantKind: KindUnavailable},
				{name: "variant", dbusErr: iwdbus.ErrDBusVariant, wantKind: KindUnavailable},
				// Daemon does not use properties; property errors are not treated as daemon unavailable.
				{name: "property", dbusErr: iwdbus.ErrDBusProperty, wantKind: KindOperationFailed},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					f := fakeIwdbusDaemonWithInfo(&iwdbus.DaemonInfo{
						Version:        "1",
						StateDirectory: "/tmp",
					})
					f.setErr(tc.dbusErr)
					d := NewDaemon(f)

					_, err := d.Info(context.Background())
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, ResourceDaemon, ce.Resource)

					require.True(t, errors.Is(err, ErrCore))
					require.True(t, errors.Is(err, tc.dbusErr))
				})
			}
		})

		t.Run("InvalidFields", func(t *testing.T) {
			tests := []struct {
				name    string
				info    *iwdbus.DaemonInfo
				wantMsg string
			}{
				{name: "empty version", info: &iwdbus.DaemonInfo{Version: "", StateDirectory: "/x"}, wantMsg: "Version"},
				{name: "version only whitespace", info: &iwdbus.DaemonInfo{Version: "   ", StateDirectory: "/x"}, wantMsg: "Version"},
				{name: "empty state directory", info: &iwdbus.DaemonInfo{Version: "1", StateDirectory: ""}, wantMsg: "StateDirectory"},
				{name: "state directory whitespace", info: &iwdbus.DaemonInfo{Version: "1", StateDirectory: "   "}, wantMsg: "StateDirectory"},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					d := NewDaemon(fakeIwdbusDaemonWithInfo(tc.info))

					_, err := d.Info(context.Background())
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, KindInvalidState, ce.Kind)
					require.Equal(t, ResourceDaemon, ce.Resource)
					require.Contains(t, err.Error(), tc.wantMsg)
				})
			}
		})

		t.Run("Success", func(t *testing.T) {
			tests := []struct {
				name string
				in   *iwdbus.DaemonInfo
				want *DaemonInfo
			}{
				{
					name: "basic",
					in:   &iwdbus.DaemonInfo{Version: "1.0.0", StateDirectory: "/iwd/state", NetworkConfigurationEnabled: true},
					want: &DaemonInfo{Version: "1.0.0", StateDirectory: "/iwd/state", NetworkConfigurationEnabled: true},
				},
				{
					name: "trim whitespace",
					in:   &iwdbus.DaemonInfo{Version: " 1.0 ", StateDirectory: "   /path/to/state   ", NetworkConfigurationEnabled: true},
					want: &DaemonInfo{Version: "1.0", StateDirectory: "/path/to/state", NetworkConfigurationEnabled: true},
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					d := NewDaemon(fakeIwdbusDaemonWithInfo(tc.in))

					out, err := d.Info(context.Background())
					require.NoError(t, err)
					require.Equal(t, tc.want, out)
				})
			}
		})
	})

	t.Run("Adapters", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			tests := []struct {
				name   string
				daemon *Daemon
			}{
				{name: "nil receiver", daemon: nil},
				{name: "inner nil", daemon: &Daemon{raw: nil}},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					_, err := tc.daemon.Adapters(context.Background())
					require.Error(t, err)
					require.True(t, errors.Is(err, ErrDaemonNotInitialized))
					require.True(t, errors.Is(err, ErrCore))
				})
			}
		})

		t.Run("DBusErrorMapping", func(t *testing.T) {
			f := &fakeIwdbusDaemon{}
			f.setErr(iwdbus.ErrDBusIntrospection)
			d := NewDaemon(f)

			_, err := d.Adapters(context.Background())
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceDaemon, ce.Resource)
			require.True(t, errors.Is(err, ErrCore))
			require.True(t, errors.Is(err, iwdbus.ErrDBusIntrospection))
		})

		t.Run("InvalidFields", func(t *testing.T) {
			tests := []struct {
				name     string
				adapters []iwdbus.AdapterRef
				wantKind Kind
				wantSub  string
			}{
				{name: "empty path", adapters: []iwdbus.AdapterRef{{Path: "", Name: "phy0"}}, wantKind: KindInvalidState, wantSub: "invalid path"},
				{name: "path not absolute", adapters: []iwdbus.AdapterRef{{Path: "not/abs", Name: "phy0"}}, wantKind: KindInvalidState, wantSub: "invalid path"},
				{name: "empty name", adapters: []iwdbus.AdapterRef{{Path: "/net/connman/iwd/phy0", Name: ""}}, wantKind: KindInvalidState, wantSub: "empty Name"},
				{name: "name whitespace", adapters: []iwdbus.AdapterRef{{Path: "/net/connman/iwd/phy0", Name: "  \t "}}, wantKind: KindInvalidState, wantSub: "empty Name"},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					f := &fakeIwdbusDaemon{}
					f.setAdapters(tc.adapters)
					d := NewDaemon(f)
					_, err := d.Adapters(context.Background())
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, ResourceAdapter, ce.Resource)
					require.Contains(t, err.Error(), tc.wantSub)
				})
			}
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusDaemon{}
			f.setAdapters([]iwdbus.AdapterRef{
				{
					Path: "/net/connman/iwd/phy0",
					Name: "phy0",
				},
				{
					Path: "  /net/connman/iwd/phy1  ",
					Name: "  phy1  ",
				},
			})
			d := NewDaemon(f)

			out, err := d.Adapters(context.Background())
			require.NoError(t, err)
			require.Equal(t, []AdapterRef{
				{Path: "/net/connman/iwd/phy0", Name: "phy0"},
				{Path: "/net/connman/iwd/phy1", Name: "phy1"},
			}, out)
		})
	})

	t.Run("ConvenienceMethods", func(t *testing.T) {
		tests := []struct {
			name string
			info *iwdbus.DaemonInfo
			run  func(t *testing.T, d *Daemon)
		}{
			{
				name: "Version",
				info: &iwdbus.DaemonInfo{Version: "2.3.4", StateDirectory: "/tmp"},
				run: func(t *testing.T, d *Daemon) {
					v, err := d.Version(context.Background())
					require.NoError(t, err)
					require.Equal(t, "2.3.4", v)
				},
			},
			{
				name: "StateDirectory",
				info: &iwdbus.DaemonInfo{Version: "1", StateDirectory: "/abc"},
				run: func(t *testing.T, d *Daemon) {
					s, err := d.StateDirectory(context.Background())
					require.NoError(t, err)
					require.Equal(t, "/abc", s)
				},
			},
			{
				name: "NetworkConfigurationEnabled",
				info: &iwdbus.DaemonInfo{Version: "1", StateDirectory: "/dir", NetworkConfigurationEnabled: false},
				run: func(t *testing.T, d *Daemon) {
					b, err := d.NetworkConfigurationEnabled(context.Background())
					require.NoError(t, err)
					require.False(t, b)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				d := NewDaemon(fakeIwdbusDaemonWithInfo(tc.info))
				tc.run(t, d)
			})
		}
	})

	t.Run("Concurrency", func(t *testing.T) {
		d := newTestDaemon(t)
		ctx := context.Background()

		var wg sync.WaitGroup
		for range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				info, err := d.Info(ctx)
				require.NoError(t, err)
				require.Equal(t, "1.0", info.Version)
			}()
		}

		wg.Wait()
	})

	t.Run("ErrorMessageStability", func(t *testing.T) {
		d := NewDaemon(fakeIwdbusDaemonWithInfo(&iwdbus.DaemonInfo{
			Version:        "1",
			StateDirectory: "",
		}))
		_, err := d.Info(context.Background())
		require.Error(t, err)

		msg := err.Error()
		require.Contains(t, msg, "invalid state")
		require.Contains(t, msg, "Daemon.Info")
		require.Contains(t, msg, "StateDirectory")
	})
}
