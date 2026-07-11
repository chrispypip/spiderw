//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func validSignalConfig(changed func(level int)) SignalLevelConfig {
	return SignalLevelConfig{Thresholds: []int{-60, -70, -80}, Changed: changed}
}

func TestSignalLevelAgent_Core(t *testing.T) {
	t.Parallel()

	t.Run("Validate", func(t *testing.T) {
		t.Parallel()
		t.Run("Valid", testSignalLevelCore_Validate_Valid)
		t.Run("NilChanged", testSignalLevelCore_Validate_NilChanged)
		t.Run("EmptyThresholds", testSignalLevelCore_Validate_EmptyThresholds)
		t.Run("NotDescending", testSignalLevelCore_Validate_NotDescending)
		t.Run("OutOfRange", testSignalLevelCore_Validate_OutOfRange)
	})

	t.Run("Handler", func(t *testing.T) {
		t.Parallel()
		t.Run("ChangedForwardsLevel", testSignalLevelCore_Handler_Changed)
		t.Run("ReleaseForwards", testSignalLevelCore_Handler_Release)
		t.Run("NilCallbacksSafe", testSignalLevelCore_Handler_NilCallbacksSafe)
	})

	t.Run("Levels", testSignalLevelCore_Levels)

	t.Run("Lifecycle", func(t *testing.T) {
		t.Parallel()
		t.Run("Unregister", testSignalLevelCore_Unregister)
		t.Run("UnregisterErrors", testSignalLevelCore_UnregisterErrors)
		t.Run("UnregisterIdempotent", testSignalLevelCore_UnregisterIdempotent)
		t.Run("UnregisterUnbound", testSignalLevelCore_UnregisterUnbound)
		t.Run("UnregisterNilReceiver", testSignalLevelCore_UnregisterNilReceiver)
	})
}

func testSignalLevelCore_Validate_Valid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validSignalConfig(func(int) {}).Validate("op"))
}

func testSignalLevelCore_Validate_NilChanged(t *testing.T) {
	t.Parallel()
	err := SignalLevelConfig{Thresholds: []int{-60}}.Validate("op")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCore)
	var ce *Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, KindInvalidArgument, ce.Kind)
	require.Equal(t, ResourceStation, ce.Resource)
}

func testSignalLevelCore_Validate_EmptyThresholds(t *testing.T) {
	t.Parallel()
	err := SignalLevelConfig{Changed: func(int) {}}.Validate("op")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrCore)
	require.Contains(t, err.Error(), "at least one signal-strength threshold")
}

func testSignalLevelCore_Validate_NotDescending(t *testing.T) {
	t.Parallel()
	for _, thresh := range [][]int{
		{-70, -60},      // ascending
		{-60, -60},      // equal (not strictly descending)
		{-60, -80, -70}, // out of order
	} {
		err := SignalLevelConfig{Thresholds: thresh, Changed: func(int) {}}.Validate("op")
		require.Error(t, err)
		require.Contains(t, err.Error(), "strictly descending")
	}
}

func testSignalLevelCore_Validate_OutOfRange(t *testing.T) {
	t.Parallel()
	// iwd's thresholds are int16, so both overflow directions are rejected.
	for _, thresh := range [][]int{
		{40000},  // above math.MaxInt16
		{-40000}, // below math.MinInt16
	} {
		err := SignalLevelConfig{Thresholds: thresh, Changed: func(int) {}}.Validate("op")
		require.Error(t, err)
		require.Contains(t, err.Error(), "out of range")
	}
}

func testSignalLevelCore_Handler_Changed(t *testing.T) {
	t.Parallel()
	var (
		called bool
		got    int
	)
	_, h := NewSignalLevelAgent(validSignalConfig(func(level int) { called = true; got = level }))
	h.Changed("/net/connman/iwd/0/3", 2)
	require.True(t, called)
	require.Equal(t, 2, got)
}

func testSignalLevelCore_Handler_Release(t *testing.T) {
	t.Parallel()
	var released bool
	cfg := validSignalConfig(func(int) {})
	cfg.Released = func() { released = true }
	_, h := NewSignalLevelAgent(cfg)
	h.Release()
	require.True(t, released)
}

func testSignalLevelCore_Handler_NilCallbacksSafe(t *testing.T) {
	t.Parallel()
	// A nil Released (and, defensively, a nil Changed) must not panic.
	_, h := NewSignalLevelAgent(SignalLevelConfig{Thresholds: []int{-60}})
	require.NotPanics(t, func() {
		h.Changed("/net/connman/iwd/0/3", 1)
		h.Release()
	})
}

func testSignalLevelCore_Levels(t *testing.T) {
	t.Parallel()
	a, _ := NewSignalLevelAgent(validSignalConfig(func(int) {}))
	require.Equal(t, []int16{-60, -70, -80}, a.Levels())
	// The returned slice is a copy; mutating it must not affect stored levels.
	got := a.Levels()
	got[0] = 0
	require.Equal(t, []int16{-60, -70, -80}, a.Levels())
}

func testSignalLevelCore_Unregister(t *testing.T) {
	t.Parallel()
	reg := &fakeSignalLevelRegistrar{}
	var unexported bool
	a, _ := NewSignalLevelAgent(validSignalConfig(func(int) {}))
	a.Bind(reg, "/spiderw/signalagent", func() error { unexported = true; return nil })

	require.NoError(t, a.Unregister(context.Background()))
	require.Equal(t, []dbus.ObjectPath{"/spiderw/signalagent"}, reg.unregisterCalls)
	require.True(t, unexported)
}

func testSignalLevelCore_UnregisterErrors(t *testing.T) {
	t.Parallel()
	reg := &fakeSignalLevelRegistrar{unregisterErr: errors.New("unreg boom")}
	a, _ := NewSignalLevelAgent(validSignalConfig(func(int) {}))
	a.Bind(reg, "/spiderw/signalagent", func() error { return errors.New("unexport boom") })

	err := a.Unregister(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "unreg boom")
	require.ErrorContains(t, err, "unexport boom")

	var ce *Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, ResourceStation, ce.Resource)
}

func testSignalLevelCore_UnregisterIdempotent(t *testing.T) {
	t.Parallel()
	reg := &fakeSignalLevelRegistrar{}
	a, _ := NewSignalLevelAgent(validSignalConfig(func(int) {}))
	a.Bind(reg, "/spiderw/signalagent", func() error { return nil })

	require.NoError(t, a.Unregister(context.Background()))
	require.NoError(t, a.Unregister(context.Background()))
	require.Equal(t, 1, reg.unregisterCount())
}

func testSignalLevelCore_UnregisterUnbound(t *testing.T) {
	t.Parallel()
	// An agent that was never bound (export/register failed) unregisters as a
	// no-op rather than erroring.
	a, _ := NewSignalLevelAgent(validSignalConfig(func(int) {}))
	require.NoError(t, a.Unregister(context.Background()))
}

func testSignalLevelCore_UnregisterNilReceiver(t *testing.T) {
	t.Parallel()
	var a *SignalLevelAgent
	err := a.Unregister(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSignalLevelAgentNotInitialized)
}
