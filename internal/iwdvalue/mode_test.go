//go:build unit

package iwdvalue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMode(t *testing.T) {
	t.Run("ParseValid", func(t *testing.T) {
		tests := []struct {
			input string
			want  Mode
		}{
			{input: "station", want: ModeStation},
			{input: "ap", want: ModeAP},
			{input: "ad-hoc", want: ModeAdHoc},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				got, ok := ParseMode(tt.input)
				require.True(t, ok)
				require.Equal(t, tt.want, got)
				require.True(t, ValidMode(got))
				require.Equal(t, tt.input, got.String())
			})
		}
	})

	t.Run("ParseInvalid", func(t *testing.T) {
		tests := []string{"", "adhoc", "STATION", "bad-mode"}

		for _, input := range tests {
			t.Run(input, func(t *testing.T) {
				got, ok := ParseMode(input)
				require.False(t, ok)
				require.Equal(t, ModeUnknown, got)
				require.False(t, ValidMode(got))
			})
		}
	})

	t.Run("UnknownString", func(t *testing.T) {
		require.Equal(t, "unknown", ModeUnknown.String())
		require.Equal(t, "unknown", Mode("bad-mode").String())
	})
}
