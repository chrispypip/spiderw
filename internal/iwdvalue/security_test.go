//go:build unit

package iwdvalue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecurityType(t *testing.T) {
	t.Run("ParseValid", func(t *testing.T) {
		tests := []struct {
			input string
			want  SecurityType
		}{
			{input: "open", want: SecurityTypeOpen},
			{input: "wep", want: SecurityTypeWEP},
			{input: "psk", want: SecurityTypePSK},
			{input: "8021x", want: SecurityType8021x},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				got, ok := ParseSecurityType(tt.input)
				require.True(t, ok)
				require.Equal(t, tt.want, got)
				require.True(t, ValidSecurityType(got))
				require.Equal(t, tt.input, got.String())
			})
		}
	})

	t.Run("ParseInvalid", func(t *testing.T) {
		tests := []string{"", "OPEN", "wpa", "eap", "bad-type"}

		for _, input := range tests {
			t.Run(input, func(t *testing.T) {
				got, ok := ParseSecurityType(input)
				require.False(t, ok)
				require.Equal(t, SecurityTypeUnknown, got)
				require.False(t, ValidSecurityType(got))
			})
		}
	})

	t.Run("UnknownString", func(t *testing.T) {
		require.Equal(t, "unknown", SecurityTypeUnknown.String())
		require.Equal(t, "unknown", SecurityType("bad-type").String())
	})
}
