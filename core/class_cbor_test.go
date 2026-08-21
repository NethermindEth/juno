package core_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cborlite"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sierraClassHash     = "0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc"
	deprecatedClassHash = "0x07db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909"

	declaredClassAtSize = 8
)

func loadDeclaredClass(
	tb testing.TB,
	network *networks.Network,
	classHash string,
) *core.DeclaredClassDefinition {
	tb.Helper()

	gw := adaptfeeder.New(feeder.NewTestClient(tb, network))
	hash := felt.UnsafeFromString[felt.Felt](classHash)
	class, err := gw.Class(tb.Context(), &hash)
	require.NoError(tb, err)

	return &core.DeclaredClassDefinition{At: 1234, Class: class}
}

func declaredClassFixtures(tb testing.TB) []struct {
	name     string
	declared *core.DeclaredClassDefinition
} {
	tb.Helper()

	return []struct {
		name     string
		declared *core.DeclaredClassDefinition
	}{
		{
			name:     "sierra",
			declared: loadDeclaredClass(tb, &networks.Integration, sierraClassHash),
		},
		{
			name:     "deprecatedCairo",
			declared: loadDeclaredClass(tb, &networks.Sepolia, deprecatedClassHash),
		},
	}
}

func decodeGeneric(data []byte, out *core.DeclaredClassDefinition) error {
	var payload []byte
	if err := encoder.Unmarshal(data, &payload); err != nil {
		return err
	}
	if len(payload) < declaredClassAtSize {
		return errors.New("payload too short to hold a declared class")
	}

	out.At = binary.BigEndian.Uint64(payload[:declaredClassAtSize])
	return encoder.Unmarshal(payload[declaredClassAtSize:], &out.Class)
}

func BenchmarkDeclaredClassUnmarshal(b *testing.B) {
	for _, fixture := range declaredClassFixtures(b) {
		stored, err := encoder.Marshal(fixture.declared)
		require.NoError(b, err)
		require.Equal(b, byte(2), stored[0]>>5, "want a CBOR byte string (major 2)")

		b.Run(fixture.name+"/cborlite", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var declared core.DeclaredClassDefinition
				if err := cborlite.Unmarshal(stored, &declared); err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fixture.name+"/generic", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				var declared core.DeclaredClassDefinition
				if err := decodeGeneric(stored, &declared); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func FuzzDeclaredClassMatchesTheGenericDecoder(f *testing.F) {
	for _, fixture := range declaredClassFixtures(f) {
		data, err := encoder.Marshal(fixture.declared)
		require.NoError(f, err)
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var got core.DeclaredClassDefinition
		if err := cborlite.Unmarshal(data, &got); err != nil {
			return
		}

		var generic core.DeclaredClassDefinition
		if err := decodeGeneric(data, &generic); err != nil {
			return
		}
		assert.Equal(t, &generic, &got)
	})
}
