package iwdmock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

// mockImportPath is the Go import path of the iwd mock binary. Building by
// import path (rather than an absolute filesystem path) lets the mock be built
// from any working directory within the module, so tests are not tied to a
// specific checkout location.
const mockImportPath = "github.com/chrispypip/spiderw/tools/test-mocks/iwdmock"

type runningMock struct {
	cmd  *exec.Cmd
	done <-chan error
}

// The iwd mock binary is built once per test process and shared by every test;
// only the mock *process* is spawned fresh per test, preserving isolation.
// Building once avoids re-invoking `go build` on every StartMock* call, which
// otherwise dominated the integration suite's runtime.
var (
	mockBinOnce sync.Once
	mockBinPath string
	errMockBin  error
)

// ensureMockBinary builds the iwd mock binary once and returns its path.
//
// The build output lives in a temporary directory that is intentionally not
// removed: it is reused for the lifetime of the test process and reclaimed by
// the OS afterward.
func ensureMockBinary() (string, error) {
	mockBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "iwdmock-bin-")
		if err != nil {
			errMockBin = err
			return
		}
		path := filepath.Join(dir, "iwdmock-bin")
		if out, err := exec.Command("go", "build", "-o", path, mockImportPath).CombinedOutput(); err != nil {
			errMockBin = fmt.Errorf("building iwd mock: %w: %s", err, out)
			return
		}
		mockBinPath = path
	})
	return mockBinPath, errMockBin
}

func waitForBusName(conn *dbus.Conn, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var names []string
		err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
		if err == nil {
			if slices.Contains(names, name) {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("timeout waiting for bus name ownership: " + name)
}

func waitForBusNameWithT(t *testing.T, name string, timeout time.Duration) (*dbus.Conn, error) {
	t.Helper()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	err = waitForBusName(conn, name, timeout)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func waitForBusNameNoT(name string, timeout time.Duration) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	err = waitForBusName(conn, name, timeout)
	if err != nil {
		return err
	}
	return err
}

// startMock spawns a fresh mock process from the shared, pre-built binary, which
// is built once by ensureMockBinary rather than per call.
func startMock(args []string) (*runningMock, error) {
	binPath, err := ensureMockBinary()
	if err != nil {
		return nil, err
	}

	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr == "" {
		return nil, errors.New("DBUS_SESSION_BUS_ADDRESS not set in test environment")
	}

	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+addr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ready := make(chan struct{})
	var readyOnce sync.Once
	signalReady := func() {
		readyOnce.Do(func() {
			close(ready)
		})
	}
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.Contains(sc.Text(), "READY") {
				signalReady()
			}
		}
	}

	go scan(stdout)
	go scan(stderr)

	select {
	case <-ready:
		return &runningMock{cmd: cmd, done: done}, nil
	case err := <-done:
		return nil, fmt.Errorf("mock exited before signaling READY: %w", err)
	case <-time.After(2 * time.Second):
		stopMock(&runningMock{cmd: cmd, done: done})
		return nil, errors.New("mock did not signal READY")
	}
}

func stopMock(mock *runningMock) {
	if mock == nil {
		return
	}
	if mock.cmd == nil || mock.cmd.Process == nil {
		return
	}

	_ = mock.cmd.Process.Signal(os.Interrupt)
	select {
	case <-mock.done:
	case <-time.After(2 * time.Second):
		_ = mock.cmd.Process.Kill()
		<-mock.done
	}
}

func startMockWithT(t *testing.T, args []string) {
	t.Helper()

	mock, err := startMock(args)
	require.NoError(t, err)
	t.Cleanup(func() {
		stopMock(mock)
	})

	conn, err := waitForBusNameWithT(t, "net.connman.iwd", 3*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})
}

func startMockNoT(args []string) error {
	mock, err := startMock(args)
	if err != nil {
		return err
	}

	err = waitForBusNameNoT("net.connman.iwd", 3*time.Second)
	if err != nil {
		stopMock(mock)
		return err
	}
	return nil
}

// StartMockNormal starts the default iwd mock for tests.
func StartMockNormal(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{})
}

// StartMockNormalNoT starts the default iwd mock without a testing.T helper.
func StartMockNormalNoT() error {
	return startMockNoT([]string{})
}

// StartMockWithOmittedOptionals starts an iwd mock that omits optional daemon fields.
func StartMockWithOmittedOptionals(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-optionals"})
}

// StartMockFirehose starts an iwd mock that emits high-frequency signals.
func StartMockFirehose(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--firehose-signals"})
}

// StartMockWithoutDaemon starts an iwd mock without exporting the daemon object.
func StartMockWithoutDaemon(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-daemon"})
}

// StartMockWithoutDaemonNoT starts a no-daemon iwd mock without a testing.T helper.
func StartMockWithoutDaemonNoT() error {
	return startMockNoT([]string{"--omit-daemon"})
}

// StartMockWithMissingDaemonInfoFields starts a mock with incomplete daemon info.
func StartMockWithMissingDaemonInfoFields(t *testing.T, version, statedir, netconf bool) {
	t.Helper()
	args := make([]string, 0)
	if version {
		args = append(args, "--omit-daemon-version")
	}
	if statedir {
		args = append(args, "--omit-daemon-statedir")
	}
	if netconf {
		args = append(args, "--omit-daemon-netconf")
	}
	startMockWithT(t, args)
}

// StartMockWithBadDaemonInfoFields starts a mock with invalid daemon info field types.
func StartMockWithBadDaemonInfoFields(t *testing.T, version, statedir, netconf bool) {
	t.Helper()
	args := make([]string, 0)
	if version {
		args = append(args, "--daemon-bad-version")
	}
	if statedir {
		args = append(args, "--daemon-bad-statedir")
	}
	if netconf {
		args = append(args, "--daemon-bad-netconf")
	}
	startMockWithT(t, args)
}

// StartMockWithExtraDaemonInfoFields starts a mock with extra daemon info fields.
func StartMockWithExtraDaemonInfoFields(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--daemon-extra-field"})
}

// StartMockWithDaemonGetInfoReturningBadType starts a mock whose GetInfo returns an invalid type.
func StartMockWithDaemonGetInfoReturningBadType(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--daemon-bad-payload"})
}

// StartMockWithDaemonFailingCalls starts a mock whose daemon methods fail.
func StartMockWithDaemonFailingCalls(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--daemon-fail-calls"})
}

// StartMockAdapterWithBadModes starts a mock adapter with invalid SupportedModes data.
func StartMockAdapterWithBadModes(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--adapter-bad-modes"})
}

// StartMockWithoutDevice starts an iwd mock that does not export a device object,
// which exercises empty device enumeration.
func StartMockWithoutDevice(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-device"})
}

// StartMockWithoutBSS starts an iwd mock that does not export a basic service set
// object, which exercises empty BSS enumeration.
func StartMockWithoutBSS(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-bss"})
}

// StartMockWithoutNetwork starts an iwd mock that does not export any network
// objects, which exercises empty network enumeration.
func StartMockWithoutNetwork(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-network"})
}

// StartMockWithoutKnownNetwork starts an iwd mock that does not export any
// known-network objects, which exercises empty known-network enumeration.
func StartMockWithoutKnownNetwork(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-knownnetwork"})
}

// StartMockWithoutAgent starts an iwd mock that does not export the AgentManager
// interface, which exercises the client's "agent manager unavailable" path.
func StartMockWithoutAgent(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-agent"})
}

// StartMockWithoutStation starts an iwd mock that does not export the Station
// interface on the station-mode device, which exercises the client's "station
// unavailable" and empty station-enumeration paths.
func StartMockWithoutStation(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-station"})
}

// StartMockWithoutWSC starts the mock with the SimpleConfiguration (WSC)
// interface omitted from the station-mode device, so the station exists but WSC
// is unavailable (as with a driver that does not support it).
func StartMockWithoutWSC(t *testing.T) {
	t.Helper()
	startMockWithT(t, []string{"--omit-wsc"})
}
