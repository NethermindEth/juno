package value_test

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/stretchr/testify/require"
)

const count = 1000

func runLiteralTest[S value.Serializer[V], V any](
	t *testing.T,
	_ S,
	value V,
	expected string,
) {
	t.Helper()
	t.Run(fmt.Sprintf("%T", *new(S)), func(t *testing.T) {
		var s S

		actualBytes, err := s.Marshal(&value)
		require.NoError(t, err)
		require.Equal(t, expected, hex.EncodeToString(actualBytes))

		var unmarshalled V
		require.NoError(t, s.Unmarshal(actualBytes, &unmarshalled))
		require.Equal(t, value, unmarshalled)
	})
}

func runRandomTest[S value.Serializer[V], V any](
	t *testing.T,
	_ S,
	generator func() V,
) {
	t.Helper()
	t.Run(fmt.Sprintf("%T", *new(S)), func(t *testing.T) {
		var s S
		for range count {
			marshalled := generator()

			valueBytes, err := s.Marshal(&marshalled)
			require.NoError(t, err)

			var unmarshalled V
			require.NoError(t, s.Unmarshal(valueBytes, &unmarshalled))

			require.Equal(t, marshalled, unmarshalled)
		}
	})
}

func TestValueSerializer(t *testing.T) {
	t.Run("Literal tests", func(t *testing.T) {
		runLiteralTest(t, value.Uint64, 2025112020251120, "000731d42298f1f0")

		feltInput := felt.FromUint64[felt.Felt](2025112120251121)
		feltOutput := "000000000000000000000000000000000000000000000000000731d4288ed2f1"

		runLiteralTest(t, value.Felt, feltInput, feltOutput)

		runLiteralTest(t, value.Hash, felt.Hash(feltInput), feltOutput)

		runLiteralTest(t, value.ClassHash, felt.ClassHash(feltInput), feltOutput)

		runLiteralTest(t, value.CasmClassHash, felt.CasmClassHash(feltInput), feltOutput)

		runLiteralTest(t, value.Bytes, []byte("hello"), "68656c6c6f")

		runLiteralTest(
			t,
			value.Binary[db.BlockNumIndexKey](),
			db.BlockNumIndexKey{
				Number: 2025112220251122,
				Index:  2025112320251123,
			},
			"000731d42e84b3f2000731d4347a94f3",
		)

		runLiteralTest(
			t,
			value.Cbor[core.L1Head](),
			core.L1Head{
				BlockNumber: 2025112420251124,
				BlockHash:   felt.NewFromUint64[felt.Felt](2025112520251125),
				StateRoot:   felt.NewFromUint64[felt.Felt](2025112620251126),
			},
			//nolint:lll,nolintlint // Ignore lll for literals, nolintlint because main config doesn't check tests
			"a369426c6f636b48617368841bff19c577f33521621bffffffffffffffff1bffffffffffffffff1b00b61cf726873781695374617465526f6f74841bff19c577347901421bffffffffffffffff1bffffffffffffffff1b00b61cea7c0915616b426c6f636b4e756d6265721b000731d43a7075f4",
		)
	})

	t.Run("Random tests", func(t *testing.T) {
		runRandomTest(t, value.Uint64, rand.Uint64)

		runRandomTest(t, value.Felt, felt.Random[felt.Felt])

		runRandomTest(t, value.Hash, felt.Random[felt.Hash])

		runRandomTest(t, value.ClassHash, felt.Random[felt.ClassHash])

		runRandomTest(t, value.CasmClassHash, felt.Random[felt.CasmClassHash])

		runRandomTest(t, value.Bytes, func() []byte {
			return []byte(cryptorand.Text())
		})

		runRandomTest(t, value.Binary[db.BlockNumIndexKey](), func() db.BlockNumIndexKey {
			return db.BlockNumIndexKey{
				Number: rand.Uint64(),
				Index:  rand.Uint64(),
			}
		})

		runRandomTest(t, value.Cbor[core.L1Head](), func() core.L1Head {
			return core.L1Head{
				BlockNumber: rand.Uint64(),
				BlockHash:   felt.NewRandom[felt.Felt](),
				StateRoot:   felt.NewRandom[felt.Felt](),
			}
		})
	})
}
