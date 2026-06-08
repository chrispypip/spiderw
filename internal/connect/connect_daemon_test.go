//go:build unit

package connect

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// NOTE: These tests intentionally do NOT run in parallel because they mutate
// package-level seam variables in connect.

func TestConnect_Daemon_CleanupCalledExactlyOnce_FailurePaths(t *testing.T) {
	type busCase struct {
		name   string
		system bool
		call   func(ctx context.Context) (*Wiring, error)
	}
	buses := []busCase{
		{name: "system", system: true, call: System},
		{name: "session", system: false, call: Session},
	}

	type scenario struct {
		name         string
		setup        func(t *testing.T, daemonErr error)
		cleanupErr   error
		assertJoined bool
	}

	scenarios := []scenario{
		{
			name: "new_iwd_daemon_error_calls_cleanup_once",
			setup: func(t *testing.T, daemonErr error) {
				newIwdDaemonFn = func(ctx context.Context, conn *dbus.Conn) (*iwdbus.Daemon, error) {
					return nil, daemonErr
				}
			},
		},
		{
			name: "new_iwd_daemon_nil_calls_cleanup_once",
			setup: func(t *testing.T, cleanupErr error) {
				newIwdDaemonFn = func(ctx context.Context, conn *dbus.Conn) (*iwdbus.Daemon, error) {
					return nil, nil
				}
			},
		},
		{
			name: "new_core_daemon_nil_calls_cleanup_once",
			setup: func(t *testing.T, cleanupErr error) {
				newIwdDaemonFn = func(ctx context.Context, conn *dbus.Conn) (*iwdbus.Daemon, error) {
					return &iwdbus.Daemon{}, nil
				}
				newCoreDaemonFn = func(raw *iwdbus.Daemon) *core.Daemon {
					return nil
				}
			},
		},
		{
			name:         "cleanup_error_is_joined_and_cleanup_called_once",
			cleanupErr:   errors.New("cleanup failed"),
			assertJoined: true,
			setup: func(t *testing.T, daemonErr error) {
				newIwdDaemonFn = func(ctx context.Context, conn *dbus.Conn) (*iwdbus.Daemon, error) {
					return nil, daemonErr
				}
			},
		},
	}

	for _, bc := range buses {
		t.Run(bc.name, func(t *testing.T) {
			for _, sc := range scenarios {
				t.Run(sc.name, func(t *testing.T) {
					// --- Save originals (all seams we might touch) ---
					origConnectSystem := connectSystemBusFn
					origConnectSession := connectSessionBusFn
					origNewIwdDaemon := newIwdDaemonFn
					origNewCoreDaemon := newCoreDaemonFn
					origCloseConn := closeConnFn
					t.Cleanup(func() {
						connectSystemBusFn = origConnectSystem
						connectSessionBusFn = origConnectSession
						newIwdDaemonFn = origNewIwdDaemon
						newCoreDaemonFn = origNewCoreDaemon
						closeConnFn = origCloseConn
					})

					// --- Arrange: connect succeeds for the chosen bus ---
					if bc.system {
						connectSystemBusFn = func(opts ...dbus.ConnOption) (*dbus.Conn, error) { return nil, nil }
					} else {
						connectSessionBusFn = func(opts ...dbus.ConnOption) (*dbus.Conn, error) { return nil, nil }
					}

					// --- Arrange: always count cleanup attempts; scenario chooses error return ---
					var closeCalls atomic.Int64
					closeErr := sc.cleanupErr // capture for closure
					closeConnFn = func(c *dbus.Conn) error {
						closeCalls.Add(1)
						return closeErr
					}

					// Stable instance for errors.Is assertions when needed.
					daemonErr := errors.New("daemon init failed")

					// Scenario-specific overrides
					sc.setup(t, daemonErr)

					// --- Act ---
					_, err := bc.call(context.Background())
					require.Error(t, err)

					// --- Assert: cleanup called exactly once ---
					got := int(closeCalls.Load())
					require.Equal(t, 1, got)

					// --- Assert: joined errors when cleanup fails ---
					if sc.assertJoined {
						require.ErrorIs(t, err, daemonErr, "expected errors.Is(err, daemonErr) ture; err=%v", err)
						require.ErrorIs(t, err, sc.cleanupErr, "expected errors.Is(err, cleanupErr) true; err=%v", err)
					}
				})
			}
		})
	}
}
