//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestDaemon_Public(t *testing.T) {
	ctx := context.Background()

	t.Run("Info", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreDaemon{}
			f.setInfo(&core.DaemonInfo{
				Version:                     "v1",
				StateDirectory:              "/state",
				NetworkConfigurationEnabled: true,
			})

			d := &Daemon{core: f}
			out, err := d.Info(ctx)
			require.NoError(t, err)
			require.Equal(t, "v1", out.Version)
			require.Equal(t, "/state", out.StateDirectory)
			require.True(t, out.NetworkConfigurationEnabled)
		})

		t.Run("MapsCoreErrors", func(t *testing.T) {
			tests := []struct {
				name         string
				coreKind     core.Kind
				coreResource core.Resource
				want         error
				resource     Resource
			}{
				{"daemon unavailable", core.KindUnavailable, core.ResourceDaemon, ErrUnavailable, ResourceDaemon},
				{"invalid state", core.KindInvalidState, core.ResourceDaemon, ErrInvalidState, ResourceDaemon},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					base := errors.New("boom")

					f := &fakeCoreDaemon{}
					f.setErr(&core.Error{
						Kind:     tc.coreKind,
						Resource: tc.coreResource,
						Op:       "core-op",
						Err:      base,
					})

					d := &Daemon{core: f}
					_, err := d.Info(ctx)
					require.Error(t, err)

					require.ErrorIs(t, err, tc.want)

					var ce *core.Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.coreKind, ce.Kind)
					require.ErrorIs(t, err, base)

					var pe *Error
					require.ErrorAs(t, err, &pe)
					require.Equal(t, tc.resource, pe.Resource)
				})
			}
		})

		t.Run("UnknownErrorBecomesInternal", func(t *testing.T) {
			base := errors.New("weird")
			f := &fakeCoreDaemon{}
			f.setErr(base)
			d := &Daemon{core: f}

			_, err := d.Info(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
			require.ErrorIs(t, err, base)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			var d *Daemon
			_, err := d.Info(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("RepeatedConsistency", func(t *testing.T) {
			f := &fakeCoreDaemon{}
			f.setInfo(&core.DaemonInfo{
				Version:        "1",
				StateDirectory: "/state",
			})
			d := &Daemon{core: f}

			out1, err1 := d.Info(ctx)
			out2, err2 := d.Info(ctx)
			require.NoError(t, err1)
			require.NoError(t, err2)
			require.Equal(t, out1, out2)
		})
	})

	methods := []struct {
		name       string
		op         func(d *Daemon) (any, error)
		newBackend func() *fakeCoreDaemon
		wantSent   error
	}{
		{
			name: "Version",
			op:   func(d *Daemon) (any, error) { return d.Version(ctx) },
			newBackend: func() *fakeCoreDaemon {
				f := &fakeCoreDaemon{}
				f.setInfoVersion("2.3.4")
				return f
			},
		},
		{
			name: "StateDirectory",
			op:   func(d *Daemon) (any, error) { return d.StateDirectory(ctx) },
			newBackend: func() *fakeCoreDaemon {
				f := &fakeCoreDaemon{}
				f.setInfoStateDirectory("/abc")
				return f
			},
		},
		{
			name: "NetworkConfigurationEnabled",
			op:   func(d *Daemon) (any, error) { return d.NetworkConfigurationEnabled(ctx) },
			newBackend: func() *fakeCoreDaemon {
				f := &fakeCoreDaemon{}
				f.setInfoNetworkConfigurationEnaled(true)
				return f
			},
		},
	}

	t.Run("Methods", func(t *testing.T) {
		for _, m := range methods {
			t.Run(m.name, func(t *testing.T) {
				t.Run("Success", func(t *testing.T) {
					f := m.newBackend()
					d := &Daemon{core: f}
					out, err := m.op(d)
					require.NoError(t, err)
					require.NotNil(t, out)
				})

				t.Run("NilReceiver", func(t *testing.T) {
					var d *Daemon
					_, err := m.op(d)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrInternal)
				})
			})
		}

		t.Run("ErrorMapping", func(t *testing.T) {
			t.Run("KindUnavailable", func(t *testing.T) {
				f := &fakeCoreDaemon{}
				f.setErr(&core.Error{
					Kind:     core.KindUnavailable,
					Resource: core.ResourceDaemon,
				})
				d := &Daemon{core: f}
				_, err := d.Version(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrUnavailable)
			})

			t.Run("KindInvalidState", func(t *testing.T) {
				f := &fakeCoreDaemon{}
				f.setErr(&core.Error{
					Kind:     core.KindInvalidState,
					Resource: core.ResourceDaemon,
				})
				d := &Daemon{core: f}
				_, err := d.StateDirectory(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidState)
			})

			t.Run("KindOperationFailed", func(t *testing.T) {
				f := &fakeCoreDaemon{}
				f.setErr(&core.Error{
					Kind:     core.KindOperationFailed,
					Resource: core.ResourceDaemon,
				})
				d := &Daemon{core: f}
				_, err := d.NetworkConfigurationEnabled(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})
		})
	})

	t.Run("ErrorMessageStability", func(t *testing.T) {
		f := &fakeCoreDaemon{}
		f.setErr(&core.Error{
			Kind: core.KindInvalidState,
			Err:  errors.New("x"),
		})
		d := &Daemon{core: f}

		_, err := d.Version(ctx)
		require.Error(t, err)

		msg1 := err.Error()
		msg2 := err.Error()
		require.Equal(t, msg1, msg2)
	})
}

func TestDaemon_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("InfoString", func(t *testing.T) {
		require.Equal(t, "<nil>", (*DaemonInfo)(nil).String())

		info := &DaemonInfo{Version: "1.30", StateDirectory: "/var/lib/iwd", NetworkConfigurationEnabled: true}
		s := info.String()
		require.Contains(t, s, "Version: 1.30")
		require.Contains(t, s, "StateDirectory: /var/lib/iwd")
		require.Contains(t, s, "NetworkConfigurationEnabled: true")
	})

	t.Run("NewDaemon_NilCore", func(t *testing.T) {
		require.Nil(t, newDaemon(nil))
	})

	t.Run("NetworksBackendError", func(t *testing.T) {
		f := &fakeCoreDaemon{}
		f.setErr(core.WrapDaemonUnavailable("op", "boom", errors.New("x")))
		_, err := newDaemon(f).Networks(ctx)
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceDaemon, pe.Resource)
	})

	t.Run("KnownNetworksBackendError", func(t *testing.T) {
		f := &fakeCoreDaemon{}
		f.setErr(core.WrapDaemonUnavailable("op", "boom", errors.New("x")))
		_, err := newDaemon(f).KnownNetworks(ctx)
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceDaemon, pe.Resource)
	})
}

func TestDaemon_StationsMapsName(t *testing.T) {
	ctx := context.Background()
	f := &fakeCoreDaemon{}
	f.setStations([]core.StationRef{{Path: "/net/connman/iwd/0/3", Name: "wlan0"}})

	refs, err := (&Daemon{core: f}).Stations(ctx)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, "/net/connman/iwd/0/3", refs[0].Path)
	require.Equal(t, "wlan0", refs[0].Name)
}
