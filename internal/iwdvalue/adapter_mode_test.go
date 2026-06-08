//go:build unit

package iwdvalue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdapterMode(t *testing.T) {
	t.Run("ParseValid", func(t *testing.T) {
		tests := []struct {
			input string
			want  AdapterMode
		}{
			{input: "station", want: AdapterModeStation},
			{input: "ap", want: AdapterModeAP},
			{input: "ad-hoc", want: AdapterModeAdHoc},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				got, ok := ParseAdapterMode(tt.input)
				require.True(t, ok)
				require.Equal(t, tt.want, got)
				require.True(t, ValidAdapterMode(got))
				require.Equal(t, tt.input, got.String())
			})
		}
	})

	t.Run("ParseInvalid", func(t *testing.T) {
		tests := []string{"", "adhoc", "STATION", "bad-mode"}

		for _, input := range tests {
			t.Run(input, func(t *testing.T) {
				got, ok := ParseAdapterMode(input)
				require.False(t, ok)
				require.Equal(t, AdapterModeUnknown, got)
				require.False(t, ValidAdapterMode(got))
			})
		}
	})

	t.Run("UnknownString", func(t *testing.T) {
		require.Equal(t, "unknown", AdapterModeUnknown.String())
		require.Equal(t, "unknown", AdapterMode("bad-mode").String())
	})
}
