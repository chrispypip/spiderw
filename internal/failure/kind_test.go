//go:build unit

package failure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublic(t *testing.T) {
	t.Run("PreservesPublicKinds", func(t *testing.T) {
		tests := []Kind{
			KindUnavailable,
			KindInvalidState,
			KindInvalidArgument,
			KindInternal,
		}

		for _, kind := range tests {
			t.Run(string(kind), func(t *testing.T) {
				require.Equal(t, kind, Public(kind))
			})
		}
	})

	t.Run("InternalOnlyKindsBecomeInternal", func(t *testing.T) {
		require.Equal(t, KindInternal, Public(KindOperationFailed))
		require.Equal(t, KindInternal, Public(Kind("unknown kind")))
	})
}

func TestResource(t *testing.T) {
	tests := []Resource{
		ResourceUnknown,
		ResourceClient,
		ResourceDaemon,
		ResourceAdapter,
		ResourceDevice,
		ResourceStation,
		ResourceNetwork,
	}

	for _, resource := range tests {
		t.Run(string(resource), func(t *testing.T) {
			require.Equal(t, string(resource), resource.String())
		})
	}
}
