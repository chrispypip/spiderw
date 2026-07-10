//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestAgent_Core(t *testing.T) {
	t.Parallel()

	t.Run("Validate", func(t *testing.T) {
		t.Parallel()
		t.Run("NoCallbacks", testAgentCore_Validate_NoCallbacks)
		t.Run("OneCallback", testAgentCore_Validate_OneCallback)
	})

	t.Run("Handler", func(t *testing.T) {
		t.Parallel()
		t.Run("Passphrase", testAgentCore_Handler_Passphrase)
		t.Run("NilCallbackYieldsNilFunc", testAgentCore_Handler_NilCallbackYieldsNilFunc)
		t.Run("UserNameAndPassword", testAgentCore_Handler_UserNameAndPassword)
		t.Run("UserPassword", testAgentCore_Handler_UserPassword)
		t.Run("CancelAbortsInFlight", testAgentCore_Handler_CancelAbortsInFlight)
		t.Run("Release", testAgentCore_Handler_Release)
	})

	t.Run("Lifecycle", func(t *testing.T) {
		t.Parallel()
		t.Run("Unregister", testAgentCore_Unregister)
		t.Run("UnregisterErrors", testAgentCore_UnregisterErrors)
		t.Run("UnregisterIdempotent", testAgentCore_UnregisterIdempotent)
		t.Run("UnregisterUnbound", testAgentCore_UnregisterUnbound)
		t.Run("UnregisterNilReceiver", testAgentCore_UnregisterNilReceiver)
	})
}

func testAgentCore_Validate_NoCallbacks(t *testing.T) {
	t.Parallel()
	err := CredentialCallbacks{OnCancel: func(string) {}}.Validate("op")
	require.Error(t, err)
	var ce *Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, KindInvalidArgument, ce.Kind)
	require.Equal(t, ResourceAgent, ce.Resource)
}

func testAgentCore_Validate_OneCallback(t *testing.T) {
	t.Parallel()
	cc := CredentialCallbacks{Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "", nil }}
	require.NoError(t, cc.Validate("op"))
}

func testAgentCore_Handler_Passphrase(t *testing.T) {
	t.Parallel()
	var gotPath string
	_, h := NewAgent(CredentialCallbacks{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) {
			gotPath = networkPath
			return "hunter2", nil
		},
	})
	require.NotNil(t, h.RequestPassphrase)
	secret, err := h.RequestPassphrase(context.Background(), dbus.ObjectPath(testAgentNetworkPath))
	require.NoError(t, err)
	require.Equal(t, "hunter2", secret)
	require.Equal(t, testAgentNetworkPath, gotPath)
}

func testAgentCore_Handler_NilCallbackYieldsNilFunc(t *testing.T) {
	t.Parallel()
	// Only Passphrase is set; the unset request callbacks must surface as nil
	// handler funcs so the iwdbus layer declines them.
	_, h := NewAgent(CredentialCallbacks{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "x", nil },
	})
	require.NotNil(t, h.RequestPassphrase)
	require.Nil(t, h.RequestPrivateKeyPassphrase)
	require.Nil(t, h.RequestUserNameAndPassword)
	require.Nil(t, h.RequestUserPassword)
}

func testAgentCore_Handler_UserNameAndPassword(t *testing.T) {
	t.Parallel()
	_, h := NewAgent(CredentialCallbacks{
		UserNameAndPassword: func(ctx context.Context, networkPath string) (string, string, error) {
			require.Equal(t, testAgentNetworkPath, networkPath)
			return "alice", "s3cret", nil
		},
	})
	require.NotNil(t, h.RequestUserNameAndPassword)
	user, password, err := h.RequestUserNameAndPassword(context.Background(), dbus.ObjectPath(testAgentNetworkPath))
	require.NoError(t, err)
	require.Equal(t, "alice", user)
	require.Equal(t, "s3cret", password)
}

func testAgentCore_Handler_UserPassword(t *testing.T) {
	t.Parallel()
	_, h := NewAgent(CredentialCallbacks{
		UserPassword: func(ctx context.Context, networkPath, user string) (string, error) {
			require.Equal(t, testAgentNetworkPath, networkPath)
			require.Equal(t, "bob", user)
			return "pw", nil
		},
	})
	require.NotNil(t, h.RequestUserPassword)
	password, err := h.RequestUserPassword(context.Background(), dbus.ObjectPath(testAgentNetworkPath), "bob")
	require.NoError(t, err)
	require.Equal(t, "pw", password)
}

func testAgentCore_Handler_CancelAbortsInFlight(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	cancelReason := make(chan string, 1)
	var ctxErr error

	_, h := NewAgent(CredentialCallbacks{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) {
			close(started)
			<-ctx.Done()
			ctxErr = ctx.Err()
			return "", ctx.Err()
		},
		OnCancel: func(reason string) { cancelReason <- reason },
	})

	done := make(chan struct{})
	go func() {
		_, _ = h.RequestPassphrase(context.Background(), dbus.ObjectPath(testAgentNetworkPath))
		close(done)
	}()

	<-started
	h.Cancel("user-canceled")
	<-done

	require.ErrorIs(t, ctxErr, context.Canceled)
	require.Equal(t, "user-canceled", <-cancelReason)
}

func testAgentCore_Handler_Release(t *testing.T) {
	t.Parallel()
	released := make(chan struct{}, 1)
	_, h := NewAgent(CredentialCallbacks{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "", nil },
		OnRelease:  func() { released <- struct{}{} },
	})
	h.Release()
	select {
	case <-released:
	default:
		t.Fatal("OnRelease was not invoked")
	}
}

func testAgentCore_Unregister(t *testing.T) {
	t.Parallel()
	mgr := &fakeAgentManager{}
	var unexported bool
	a, _ := NewAgent(CredentialCallbacks{Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "", nil }})
	a.Bind(mgr, "/spiderw/agent", func() error { unexported = true; return nil })

	require.NoError(t, a.Unregister(context.Background()))
	require.Equal(t, []dbus.ObjectPath{"/spiderw/agent"}, mgr.unregisterCalls)
	require.True(t, unexported)
}

func testAgentCore_UnregisterErrors(t *testing.T) {
	t.Parallel()
	// Unregister cancels any in-flight request, then joins the unregister and
	// unexport failures — both wrapped as matchable core Errors.
	mgr := &fakeAgentManager{unregisterErr: errors.New("unreg boom")}
	a, _ := NewAgent(CredentialCallbacks{Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "", nil }})
	a.Bind(mgr, "/spiderw/agent", func() error { return errors.New("unexport boom") })

	canceled := false
	a.mu.Lock()
	a.currentCancel = func() { canceled = true }
	a.mu.Unlock()

	err := a.Unregister(context.Background())
	require.Error(t, err)
	require.True(t, canceled, "in-flight request should be canceled")
	require.ErrorContains(t, err, "unreg boom")
	require.ErrorContains(t, err, "unexport boom")

	var ce *Error
	require.ErrorAs(t, err, &ce)
	require.Equal(t, ResourceAgent, ce.Resource)
}

func testAgentCore_UnregisterIdempotent(t *testing.T) {
	t.Parallel()
	mgr := &fakeAgentManager{}
	a, _ := NewAgent(CredentialCallbacks{Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "", nil }})
	a.Bind(mgr, "/spiderw/agent", func() error { return nil })

	require.NoError(t, a.Unregister(context.Background()))
	require.NoError(t, a.Unregister(context.Background()))
	require.Equal(t, 1, mgr.unregisterCount())
}

func testAgentCore_UnregisterUnbound(t *testing.T) {
	t.Parallel()
	// An agent that was never bound (export/register failed) unregisters as a
	// no-op rather than erroring.
	a, _ := NewAgent(CredentialCallbacks{Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "", nil }})
	require.NoError(t, a.Unregister(context.Background()))
}

func testAgentCore_UnregisterNilReceiver(t *testing.T) {
	t.Parallel()
	var a *Agent
	err := a.Unregister(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAgentNotInitialized)
}
