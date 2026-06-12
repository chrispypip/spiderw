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
	// tmpDir is non-empty when startMock created its own temporary directory
	// for the built binary and is responsible for removing it on stop.
	tmpDir string
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

func startMock(dir string, args []string) (*runningMock, error) {
	// When no directory is supplied, build into a temporary directory owned by
	// this mock and removed in stopMock.
	ownTmp := ""
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "iwdmock-")
		if err != nil {
			return nil, err
		}
		ownTmp = dir
	}
	binPath := filepath.Join(dir, "iwdmock-bin")

	cleanupOwnTmp := func() {
		if ownTmp != "" {
			_ = os.RemoveAll(ownTmp)
		}
	}

	buildCmd := exec.Command("go", "build", "-o", binPath, mockImportPath)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		cleanupOwnTmp()
		return nil, fmt.Errorf("building iwd mock: %w: %s", err, out)
	}

	addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if addr == "" {
		cleanupOwnTmp()
		return nil, errors.New("DBUS_SESSION_BUS_ADDRESS not set in test environment")
	}
	cmd := exec.Command(binPath, args...)

	env := append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+addr)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cleanupOwnTmp()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cleanupOwnTmp()
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		cleanupOwnTmp()
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
	var scanErr error
	scan := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := sc.Text()
			if strings.Contains(line, "READY") {
				signalReady()
			}
		}
		if err := sc.Err(); err != nil {
			scanErr = err
			return
		}
	}
	if scanErr != nil {
		return nil, scanErr
	}

	go scan(stdout)
	go scan(stderr)

	select {
	case <-ready:
		return &runningMock{cmd: cmd, done: done, tmpDir: ownTmp}, nil
	case err := <-done:
		cleanupOwnTmp()
		return nil, fmt.Errorf("mock exited before signaling READY: %w", err)
	case <-time.After(2 * time.Second):
		stopMock(&runningMock{cmd: cmd, done: done, tmpDir: ownTmp})
		return nil, errors.New("mock did not signal READY")
	}
}

func stopMock(mock *runningMock) {
	if mock == nil {
		return
	}
	if mock.tmpDir != "" {
		defer func() { _ = os.RemoveAll(mock.tmpDir) }()
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

func startMockWithT(t *testing.T, dir string, args []string) {
	t.Helper()

	mock, err := startMock(dir, args)
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

func startMockNoT(dir string, args []string) error {
	mock, err := startMock(dir, args)
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
func StartMockNormal(t *testing.T, dir string) {
	startMockWithT(t, dir, []string{})
}

// StartMockNormalNoT starts the default iwd mock without a testing.T helper.
func StartMockNormalNoT(dir string) error {
	return startMockNoT(dir, []string{})
}

// StartMockWithOmittedOptionals starts an iwd mock that omits optional daemon fields.
func StartMockWithOmittedOptionals(t *testing.T) {
	startMockWithT(t, "", []string{"--omit-optionals"})
}

// StartMockFirehose starts an iwd mock that emits high-frequency signals.
func StartMockFirehose(t *testing.T) {
	startMockWithT(t, "", []string{"--firehose-signals"})
}

// StartMockWithoutDaemon starts an iwd mock without exporting the daemon object.
func StartMockWithoutDaemon(t *testing.T, dir string) {
	startMockWithT(t, dir, []string{"--omit-daemon"})
}

// StartMockWithoutDaemonNoT starts a no-daemon iwd mock without a testing.T helper.
func StartMockWithoutDaemonNoT(dir string) error {
	return startMockNoT(dir, []string{"--omit-daemon"})
}

// StartMockWithMissingDaemonInfoFields starts a mock with incomplete daemon info.
func StartMockWithMissingDaemonInfoFields(t *testing.T, version, statedir, netconf bool) {
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
	startMockWithT(t, "", args)
}

// StartMockWithBadDaemonInfoFields starts a mock with invalid daemon info field types.
func StartMockWithBadDaemonInfoFields(t *testing.T, version, statedir, netconf bool) {
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
	startMockWithT(t, "", args)
}

// StartMockWithExtraDaemonInfoFields starts a mock with extra daemon info fields.
func StartMockWithExtraDaemonInfoFields(t *testing.T) {
	startMockWithT(t, "", []string{"--daemon-extra-field"})
}

// StartMockWithDaemonGetInfoReturningBadType starts a mock whose GetInfo returns an invalid type.
func StartMockWithDaemonGetInfoReturningBadType(t *testing.T) {
	startMockWithT(t, "", []string{"--daemon-bad-payload"})
}

// StartMockWithDaemonFailingCalls starts a mock whose daemon methods fail.
func StartMockWithDaemonFailingCalls(t *testing.T) {
	startMockWithT(t, "", []string{"--daemon-fail-calls"})
}

// StartMockAdapterWithBadModes starts a mock adapter with invalid SupportedModes data.
func StartMockAdapterWithBadModes(t *testing.T) {
	startMockWithT(t, "", []string{"--adapter-bad-modes"})
}

// StartMockWithoutDevice starts an iwd mock that does not export a device object,
// which exercises empty device enumeration.
func StartMockWithoutDevice(t *testing.T, dir string) {
	startMockWithT(t, dir, []string{"--omit-device"})
}
