//go:build unit

package iwdvalue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStationState(t *testing.T) {
	t.Run("ParseValid", func(t *testing.T) {
		tests := []struct {
			input string
			want  StationState
		}{
			{input: "connected", want: StationStateConnected},
			{input: "disconnected", want: StationStateDisconnected},
			{input: "connecting", want: StationStateConnecting},
			{input: "disconnecting", want: StationStateDisconnecting},
			{input: "roaming", want: StationStateRoaming},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				got, ok := ParseStationState(tt.input)
				require.True(t, ok)
				require.Equal(t, tt.want, got)
				require.True(t, ValidStationState(got))
				require.Equal(t, tt.input, got.String())
			})
		}
	})

	t.Run("ParseInvalid", func(t *testing.T) {
		tests := []string{"", "CONNECTED", "idle", "scanning", "bad-state"}

		for _, input := range tests {
			t.Run(input, func(t *testing.T) {
				got, ok := ParseStationState(input)
				require.False(t, ok)
				require.Equal(t, StationStateUnknown, got)
				require.False(t, ValidStationState(got))
			})
		}
	})

	t.Run("UnknownString", func(t *testing.T) {
		require.Equal(t, "unknown", StationStateUnknown.String())
		require.Equal(t, "unknown", StationState("bad-state").String())
	})
}
