//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestSignalLevelAgent_Public(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	validCfg := func(cb func(int)) SignalLevelConfig {
		return SignalLevelConfig{Thresholds: []int{-60, -70, -80}, Changed: cb}
	}
	newMonitorStation := func(register func(ctx context.Context, stationPath string, cfg core.SignalLevelConfig) (core.SignalLevelAgentIface, error)) *Station {
		return newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0").withSignalMonitor(register)
	}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		var (
			gotPath   string
			gotLevels []int
			gotLevel  int
		)
		register := func(ctx context.Context, stationPath string, cfg core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			gotPath = stationPath
			gotLevels = cfg.Thresholds
			cfg.Changed(2) // simulate iwd delivering a band crossing
			return &fakeCoreSignalLevelAgent{}, nil
		}
		agent, err := newMonitorStation(register).MonitorSignalLevel(ctx, validCfg(func(l int) { gotLevel = l }))
		require.NoError(t, err)
		require.NotNil(t, agent)
		require.Equal(t, "/net/connman/iwd/0/3", gotPath)
		require.Equal(t, []int{-60, -70, -80}, gotLevels)
		require.Equal(t, 2, gotLevel, "the user callback must thread through to the core config")
	})

	t.Run("ValidationRejectedBeforeRegister", func(t *testing.T) {
		t.Parallel()
		called := false
		register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			called = true
			return &fakeCoreSignalLevelAgent{}, nil
		}
		// nil Changed fails validation.
		_, err := newMonitorStation(register).MonitorSignalLevel(ctx, SignalLevelConfig{Thresholds: []int{-60}})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
		require.False(t, called, "registration must not run when the config is invalid")
	})

	t.Run("NotDescendingRejected", func(t *testing.T) {
		t.Parallel()
		register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			return &fakeCoreSignalLevelAgent{}, nil
		}
		_, err := newMonitorStation(register).MonitorSignalLevel(ctx, SignalLevelConfig{Thresholds: []int{-70, -60}, Changed: func(int) {}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "strictly descending")
	})

	t.Run("NoWiringHook", func(t *testing.T) {
		t.Parallel()
		// A bare station (not Client-constructed) has no monitor hook.
		st := newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0")
		_, err := st.MonitorSignalLevel(ctx, validCfg(func(int) {}))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("RegisterErrorWraps", func(t *testing.T) {
		t.Parallel()
		register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			return nil, core.WrapStationUnavailable("op", "boom", errors.New("x"))
		}
		_, err := newMonitorStation(register).MonitorSignalLevel(ctx, validCfg(func(int) {}))
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceStation, pe.Resource)
	})

	t.Run("NilCoreAgentIsInternal", func(t *testing.T) {
		t.Parallel()
		// A registration hook that succeeds but yields a nil core agent surfaces
		// as an internal error rather than a nil handle.
		register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			return nil, nil
		}
		agent, err := newMonitorStation(register).MonitorSignalLevel(ctx, validCfg(func(int) {}))
		require.Nil(t, agent)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var st *Station
		_, err := st.MonitorSignalLevel(ctx, validCfg(func(int) {}))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("Unregister", func(t *testing.T) {
		t.Parallel()
		fake := &fakeCoreSignalLevelAgent{}
		register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			return fake, nil
		}
		agent, err := newMonitorStation(register).MonitorSignalLevel(ctx, validCfg(func(int) {}))
		require.NoError(t, err)
		require.NoError(t, agent.Unregister(ctx))
		require.Equal(t, int32(1), fake.unregisters.Load())
	})

	t.Run("UnregisterNilReceiver", func(t *testing.T) {
		t.Parallel()
		var a *SignalLevelAgent
		require.Error(t, a.Unregister(ctx))
	})

	t.Run("UnregisterCoreErrorWraps", func(t *testing.T) {
		t.Parallel()
		fake := &fakeCoreSignalLevelAgent{unregisterErr: core.WrapStationUnavailable("op", "boom", errors.New("x"))}
		register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			return fake, nil
		}
		agent, err := newMonitorStation(register).MonitorSignalLevel(ctx, validCfg(func(int) {}))
		require.NoError(t, err)
		err = agent.Unregister(ctx)
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceStation, pe.Resource)
	})
}
