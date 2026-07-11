//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// TestSignalAgentMock_MonitorReceivesChanged drives the full signal-level agent
// round-trip end to end: MonitorSignalLevel exports the agent object and
// registers it with the station, the mock calls back Changed with the initial
// band, and Unregister tears it down.
func TestSignalAgentMock_MonitorReceivesChanged(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	levels := make(chan int, 4)
	agent, err := station.MonitorSignalLevel(ctx, spiderw.SignalLevelConfig{
		Thresholds: []int{-60, -70, -80},
		Changed:    func(level int) { levels <- level },
	})
	require.NoError(t, err)
	require.NotNil(t, agent)

	select {
	case level := <-levels:
		require.Equal(t, 1, level) // the mock's initial band index
	case <-time.After(signalTimeout):
		t.Fatal("timed out waiting for signal level Changed callback")
	}

	require.NoError(t, agent.Unregister(ctx))
}

// TestSignalAgentMock_ReRegisterAfterUnregister verifies the agent lifecycle:
// once Unregister frees the station's slot, a fresh monitor registers and
// receives its own Changed.
func TestSignalAgentMock_ReRegisterAfterUnregister(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	monitorOnce := func() {
		got := make(chan int, 4)
		agent, err := station.MonitorSignalLevel(ctx, spiderw.SignalLevelConfig{
			Thresholds: []int{-60},
			Changed:    func(level int) { got <- level },
		})
		require.NoError(t, err)
		select {
		case <-got:
		case <-time.After(signalTimeout):
			t.Fatal("timed out waiting for Changed")
		}
		require.NoError(t, agent.Unregister(ctx))
	}

	monitorOnce()
	monitorOnce() // registering again after unregister must succeed
}

// TestSignalAgentMock_ExportObjectRoundTrip directly exercises
// iwdbus.ExportSignalLevelAgent on a real session bus: it exports the agent
// object, drives Changed and Release through the bus (not via a direct method
// call), and confirms the returned unexport removes the object.
func TestSignalAgentMock_ExportObjectRoundTrip(t *testing.T) {
	conn, err := dbus.ConnectSessionBus()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	const agentPath = dbus.ObjectPath("/spiderw/test/signalagent")
	changed := make(chan uint8, 1)
	released := make(chan struct{}, 1)

	unexport, err := iwdbus.ExportSignalLevelAgent(conn, agentPath, iwdbus.SignalLevelAgentHandler{
		Changed: func(device dbus.ObjectPath, level uint8) { changed <- level },
		Release: func() { released <- struct{}{} },
	})
	require.NoError(t, err)
	require.NotNil(t, unexport)

	obj := conn.Object(conn.Names()[0], agentPath) // call ourselves back over the bus

	require.NoError(t, obj.Call(iwdbus.IwdSignalLevelAgentIface+".Changed", 0,
		dbus.ObjectPath("/net/connman/iwd/0/3"), uint8(2)).Err)
	select {
	case level := <-changed:
		require.Equal(t, uint8(2), level)
	case <-time.After(signalTimeout):
		t.Fatal("Changed did not dispatch through the bus to the handler")
	}

	require.NoError(t, obj.Call(iwdbus.IwdSignalLevelAgentIface+".Release", 0).Err)
	select {
	case <-released:
	case <-time.After(signalTimeout):
		t.Fatal("Release did not dispatch through the bus to the handler")
	}

	require.NoError(t, unexport())
}
