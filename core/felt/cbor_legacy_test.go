//go:build !compactfelt

package felt_test

import (
	"math"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/require"
)

// The default build must write exactly what the generic encoder writes for the limbs, so that
// stored rows stay byte for byte what main stores.

// fromLimbs bypasses SetBigInt's Montgomery conversion, so a small
// argument here actually lands in a limb. SetBigInt() does not.
func fromLimbs[F felt.FeltLike](limbs ...uint64) F {
	var l [4]uint64
	copy(l[:], limbs)
	return F(fp.Element(l))
}

// requireMarshalEquivalent checks that value marshals to the generic encoder's limb array.
func requireMarshalEquivalent(t *testing.T, value *felt.Felt) []byte {
	t.Helper()

	fast, err := value.MarshalCBOR()
	require.NoError(t, err)
	generic, err := cbor.Marshal((*fp.Element)(value))
	require.NoError(t, err)
	require.Equal(t, generic, fast, "fast marshal disagrees with generic marshal")
	return fast
}

func TestLegacyCBOREncoding(t *testing.T) {
	type valueCase struct {
		name  string
		value felt.Felt
	}

	cases := []valueCase{
		{"zero", feltFromBigInt(big.NewInt(0))},
	}
	for _, boundary := range []struct {
		name string
		limb uint64
	}{
		{"one", 1},
		{"largest inline", 23},
		{"smallest 1-byte", 24},
		{"largest 1-byte (max uint8)", math.MaxUint8},
		{"smallest 2-byte", math.MaxUint8 + 1},
		{"largest 2-byte (max uint16)", math.MaxUint16},
		{"smallest 4-byte", math.MaxUint16 + 1},
		{"largest 4-byte (max uint32)", math.MaxUint32},
		{"smallest 8-byte", math.MaxUint32 + 1},
		{"largest 8-byte (max uint64)", math.MaxUint64},
	} {
		cases = append(cases, valueCase{"limb: " + boundary.name, fromLimbs[felt.Felt](boundary.limb)})
	}
	cases = append(cases, valueCase{
		"limb: boundary values spread across all four limbs",
		fromLimbs[felt.Felt](23, math.MaxUint8, math.MaxUint16, math.MaxUint64),
	})

	modulus := fp.Modulus()
	largest := new(big.Int).Sub(modulus, big.NewInt(1))
	cases = append(cases,
		valueCase{"field modulus (reduces to zero)", feltFromBigInt(modulus)},
		valueCase{"largest valid felt (modulus - 1)", feltFromBigInt(largest)},
		valueCase{"half of largest valid felt", feltFromBigInt(new(big.Int).Rsh(largest, 1))},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.value

			encoded := requireMarshalEquivalent(t, &original)
			require.Equal(t, byte(cborArrayHeader), encoded[0], "MarshalCBOR shape changed")

			var decoded felt.Felt
			require.NoError(t, decoded.UnmarshalCBOR(encoded))
			require.True(t, original.Equal(&decoded), "round trip changed the value")
		})
	}
}

// FELT_CBOR_STRESS_N=1000000000 go test ./core/felt/ -run TestLegacyCBORStress -v -timeout 2h
func TestLegacyCBORStress(t *testing.T) {
	raw := os.Getenv("FELT_CBOR_STRESS_N")
	if raw == "" {
		t.Skip("set FELT_CBOR_STRESS_N (e.g. 1000000000) to run this sweep")
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	require.NoError(t, err)

	rng := rand.New(rand.NewSource(1))
	const logEvery = 50_000_000

	for i := range n {
		value := fromLimbs[felt.Felt](rng.Uint64(), rng.Uint64(), rng.Uint64(), rng.Uint64())
		encoded := requireMarshalEquivalent(t, &value)

		var decoded felt.Felt
		require.NoError(t, decoded.UnmarshalCBOR(encoded))
		require.True(t, decoded.Equal(&value))

		if i > 0 && i%logEvery == 0 {
			t.Logf("checked %d/%d", i, n)
		}
	}
	t.Logf("checked %d random felts, 0 mismatches", n)
}

func FuzzLegacyCBORMarshalEquivalence(f *testing.F) {
	f.Add(uint64(0), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(1), uint64(0), uint64(0), uint64(0))
	f.Add(uint64(23), uint64(24), uint64(255), uint64(256))
	f.Add(uint64(65535), uint64(65536), uint64(1)<<32, uint64(1)<<63)
	f.Add(
		uint64(math.MaxUint8), uint64(math.MaxUint16),
		uint64(math.MaxUint32), uint64(math.MaxUint64),
	)
	f.Add(
		uint64(math.MaxUint64), uint64(math.MaxUint64),
		uint64(math.MaxUint64), uint64(math.MaxUint64),
	)

	f.Fuzz(func(t *testing.T, l0, l1, l2, l3 uint64) {
		value := fromLimbs[felt.Felt](l0, l1, l2, l3)
		requireMarshalEquivalent(t, &value)
	})
}

func TestStoredBytes(t *testing.T) {
	values := []felt.Felt{felt.Zero, felt.One, felt.FromUint64[felt.Felt](1 << 40), largestFelt(t)}
	for _, value := range values {
		require.Equal(t, value.Marshal(), value.StoredBytes())
	}
}
