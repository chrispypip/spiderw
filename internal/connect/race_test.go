//go:build race

package connect

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

func TestRace_Wiring_Session_Construct(t *testing.T) {
	require.NoError(t, iwdmock.StartMockNormalNoT())

	const N = 50
	errCh := make(chan error, N)

	for range N {
		go func() {
			_, err := newTestWiring(false)
			errCh <- err
		}()
	}

	for range N {
		require.NoError(t, <-errCh)
	}
}

func TestRace_Wiring_Session_CleanupIdempotent(t *testing.T) {
	require.NoError(t, iwdmock.StartMockNormalNoT())

	w, err := newTestWiring(false)
	require.NoError(t, err)
	require.NotNil(t, w)
	require.NotNil(t, w.Cleanup)

	require.NoError(t, w.Cleanup())
	require.NoError(t, w.Cleanup()) // must not panic or error
}

func TestRace_Wiring_PartialFailure_CleanupSafeSession(t *testing.T) {
	require.NoError(t, iwdmock.StartMockWithoutDaemonNoT())

	w, err := newTestWiring(false)
	require.Error(t, err)

	if w != nil && w.Cleanup != nil {
		require.NotPanics(t, func() {
			_ = w.Cleanup()
		})
	}
}

func TestRace_Wiring_Session_ConstructAndCleanup(t *testing.T) {
	require.NoError(t, iwdmock.StartMockNormalNoT())

	const N = 25
	errCh := make(chan error, N)

	for range N {
		go func() {
			w, err := newTestWiring(false)
			if err == nil && w != nil && w.Cleanup != nil {
				_ = w.Cleanup()
				_ = w.Cleanup()
			}
			errCh <- err
		}()
	}

	for range N {
		require.NoError(t, <-errCh)
	}
}

func newTestWiring(system bool) (*Wiring, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if system {
		return System(ctx)
	}
	return Session(ctx)
}
