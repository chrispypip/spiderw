//go:build unit

package iwdbus

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestAgent_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("AgentManager", func(t *testing.T) {
		t.Parallel()
		t.Run("RegisterAgent", testAgentManager_RegisterAgent)
		t.Run("UnregisterAgent", testAgentManager_UnregisterAgent)
		t.Run("RegisterAgent_ErrorMapping", testAgentManager_RegisterAgent_ErrorMapping)
		t.Run("RegisterAgent_Uninitialized", testAgentManager_RegisterAgent_Uninitialized)
		t.Run("UnregisterAgent_Uninitialized", testAgentManager_UnregisterAgent_Uninitialized)
	})

	t.Run("AgentObject", func(t *testing.T) {
		t.Parallel()
		t.Run("RequestPassphrase", testAgentObject_RequestPassphrase)
		t.Run("RequestPassphrase_NilDeclines", testAgentObject_RequestPassphrase_NilDeclines)
		t.Run("RequestPassphrase_ErrorDeclines", testAgentObject_RequestPassphrase_ErrorDeclines)
		t.Run("RequestPrivateKeyPassphrase", testAgentObject_RequestPrivateKeyPassphrase)
		t.Run("RequestUserNameAndPassword", testAgentObject_RequestUserNameAndPassword)
		t.Run("RequestUserNameAndPassword_NilDeclines", testAgentObject_RequestUserNameAndPassword_NilDeclines)
		t.Run("RequestUserPassword", testAgentObject_RequestUserPassword)
		t.Run("Cancel", testAgentObject_Cancel)
		t.Run("Release", testAgentObject_Release)
		t.Run("CancelRelease_NilSafe", testAgentObject_CancelRelease_NilSafe)
	})
}

const testNetworkPath = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/secure")

func testAgentManager_RegisterAgent(t *testing.T) {
	t.Parallel()
	var called bool
	m := &AgentManager{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
		called = true
		require.Equal(t, IwdAgentManagerIface, iface)
		require.Equal(t, "RegisterAgent", method)
		require.Len(t, args, 1)
		require.Equal(t, dbus.ObjectPath("/spiderw/agent"), args[0])
		return nil, nil
	}}}
	require.NoError(t, m.RegisterAgent(context.Background(), "/spiderw/agent"))
	require.True(t, called)
}

func testAgentManager_UnregisterAgent(t *testing.T) {
	t.Parallel()
	var called bool
	m := &AgentManager{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, _ ...interface{}) ([]interface{}, error) {
		called = true
		require.Equal(t, IwdAgentManagerIface, iface)
		require.Equal(t, "UnregisterAgent", method)
		return nil, nil
	}}}
	require.NoError(t, m.UnregisterAgent(context.Background(), "/spiderw/agent"))
	require.True(t, called)
}

func testAgentManager_RegisterAgent_ErrorMapping(t *testing.T) {
	t.Parallel()
	m := &AgentManager{intro: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
		return nil, dbus.Error{Name: IwdErrorBusy, Body: []interface{}{"busy"}}
	}}}
	err := m.RegisterAgent(context.Background(), "/spiderw/agent")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBusy)
	require.ErrorIs(t, err, ErrDBusMethod)
}

func testAgentManager_RegisterAgent_Uninitialized(t *testing.T) {
	t.Parallel()
	m := &AgentManager{}
	err := m.RegisterAgent(context.Background(), "/spiderw/agent")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAgentManagerUninitialized)
}

func testAgentManager_UnregisterAgent_Uninitialized(t *testing.T) {
	t.Parallel()
	m := &AgentManager{}
	err := m.UnregisterAgent(context.Background(), "/spiderw/agent")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAgentManagerUninitialized)
}

func testAgentObject_RequestPassphrase(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{
		RequestPassphrase: func(ctx context.Context, network dbus.ObjectPath) (string, error) {
			require.Equal(t, testNetworkPath, network)
			return "hunter2", nil
		},
	}}
	secret, derr := a.RequestPassphrase(testNetworkPath)
	require.Nil(t, derr)
	require.Equal(t, "hunter2", secret)
}

func testAgentObject_RequestPassphrase_NilDeclines(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{}}
	secret, derr := a.RequestPassphrase(testNetworkPath)
	require.Equal(t, "", secret)
	require.NotNil(t, derr)
	require.Equal(t, IwdAgentErrorCanceled, derr.Name)
}

func testAgentObject_RequestPassphrase_ErrorDeclines(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{
		RequestPassphrase: func(ctx context.Context, network dbus.ObjectPath) (string, error) {
			return "", errors.New("user dismissed prompt")
		},
	}}
	secret, derr := a.RequestPassphrase(testNetworkPath)
	require.Equal(t, "", secret)
	require.NotNil(t, derr)
	require.Equal(t, IwdAgentErrorCanceled, derr.Name)
}

func testAgentObject_RequestPrivateKeyPassphrase(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{
		RequestPrivateKeyPassphrase: func(ctx context.Context, network dbus.ObjectPath) (string, error) {
			return "keypass", nil
		},
	}}
	secret, derr := a.RequestPrivateKeyPassphrase(testNetworkPath)
	require.Nil(t, derr)
	require.Equal(t, "keypass", secret)
}

func testAgentObject_RequestUserNameAndPassword(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{
		RequestUserNameAndPassword: func(ctx context.Context, network dbus.ObjectPath) (string, string, error) {
			return "alice", "s3cret", nil
		},
	}}
	user, password, derr := a.RequestUserNameAndPassword(testNetworkPath)
	require.Nil(t, derr)
	require.Equal(t, "alice", user)
	require.Equal(t, "s3cret", password)
}

func testAgentObject_RequestUserNameAndPassword_NilDeclines(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{}}
	user, password, derr := a.RequestUserNameAndPassword(testNetworkPath)
	require.Equal(t, "", user)
	require.Equal(t, "", password)
	require.NotNil(t, derr)
	require.Equal(t, IwdAgentErrorCanceled, derr.Name)
}

func testAgentObject_RequestUserPassword(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{
		RequestUserPassword: func(ctx context.Context, network dbus.ObjectPath, user string) (string, error) {
			require.Equal(t, "bob", user)
			return "pw", nil
		},
	}}
	password, derr := a.RequestUserPassword(testNetworkPath, "bob")
	require.Nil(t, derr)
	require.Equal(t, "pw", password)
}

func testAgentObject_Cancel(t *testing.T) {
	t.Parallel()
	var got string
	a := &agentObject{handler: AgentHandler{Cancel: func(reason string) { got = reason }}}
	require.Nil(t, a.Cancel("user-canceled"))
	require.Equal(t, "user-canceled", got)
}

func testAgentObject_Release(t *testing.T) {
	t.Parallel()
	var released bool
	a := &agentObject{handler: AgentHandler{Release: func() { released = true }}}
	require.Nil(t, a.Release())
	require.True(t, released)
}

func testAgentObject_CancelRelease_NilSafe(t *testing.T) {
	t.Parallel()
	a := &agentObject{handler: AgentHandler{}}
	require.Nil(t, a.Cancel("timed-out"))
	require.Nil(t, a.Release())
}
