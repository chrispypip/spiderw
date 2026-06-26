//go:build unit

package iwdvalue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkType(t *testing.T) {
	t.Run("ParseValid", func(t *testing.T) {
		tests := []struct {
			input string
			want  NetworkType
		}{
			{input: "open", want: NetworkTypeOpen},
			{input: "wep", want: NetworkTypeWEP},
			{input: "psk", want: NetworkTypePSK},
			{input: "8021x", want: NetworkType8021x},
			{input: "hotspot", want: NetworkTypeHotspot},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				got, ok := ParseNetworkType(tt.input)
				require.True(t, ok)
				require.Equal(t, tt.want, got)
				require.True(t, ValidNetworkType(got))
				require.Equal(t, tt.input, got.String())
			})
		}
	})

	t.Run("ParseInvalid", func(t *testing.T) {
		tests := []string{"", "OPEN", "wpa", "eap", "bad-type"}

		for _, input := range tests {
			t.Run(input, func(t *testing.T) {
				got, ok := ParseNetworkType(input)
				require.False(t, ok)
				require.Equal(t, NetworkTypeUnknown, got)
				require.False(t, ValidNetworkType(got))
			})
		}
	})

	t.Run("UnknownString", func(t *testing.T) {
		require.Equal(t, "unknown", NetworkTypeUnknown.String())
		require.Equal(t, "unknown", NetworkType("bad-type").String())
	})
}
