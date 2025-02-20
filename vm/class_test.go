package vm

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

func BenchmarkSierraVersion(b *testing.B) {
	b.Run("pre 0.1.0", func(b *testing.B) {
		prog, err := new(felt.Felt).SetString("0x302e312e30")
		require.NoError(b, err)
		b.ResetTimer()
		var version string
		for i := 0; i < b.N; i++ {
			version, _ = parseSierraVersion([]*felt.Felt{prog})
		}
		_ = version
	})

	b.Run("after 0.1.0", func(b *testing.B) {
		prog := []*felt.Felt{
			utils.Ptr(felt.New(*new(fp.Element).SetUint64(0))),
			utils.Ptr(felt.New(*new(fp.Element).SetUint64(0))),
			utils.Ptr(felt.New(*new(fp.Element).SetUint64(0))),
		}
		b.ResetTimer()
		var version string
		for i := 0; i < b.N; i++ {
			version, _ = parseSierraVersion(prog)
		}
		_ = version
	})
}

func TestSierraVersion(t *testing.T) {
	t.Run("pre 0.1.0", func(t *testing.T) {
		f, err := new(felt.Felt).SetString("0x302e312e30")
		require.NoError(t, err)
		version, err := parseSierraVersion([]*felt.Felt{f})
		require.NoError(t, err)
		require.Equal(t, "0.1.0", version)
	})
	t.Run("after 0.1.0", func(t *testing.T) {
		prog := []*felt.Felt{
			utils.Ptr(felt.New(*new(fp.Element).SetUint64(1))),
			utils.Ptr(felt.New(*new(fp.Element).SetUint64(2))),
			utils.Ptr(felt.New(*new(fp.Element).SetUint64(3))),
		}

		version, err := parseSierraVersion(prog)
		require.NoError(t, err)
		require.Equal(t, "1.2.3", version)
	})
}
