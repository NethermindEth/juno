package core_test

import (
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sierraClassHash     = "0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc"
	deprecatedClassHash = "0x07db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909"
)

// decodeDeclaredClassGeneric decodes a declared class the way the reflection
// based decoder does for a type that only implements encoding.BinaryUnmarshaler.
// It is the baseline the fast path has to agree with.
func decodeDeclaredClassGeneric(tb testing.TB, data []byte) *core.DeclaredClassDefinition {
	tb.Helper()

	var payload []byte
	require.NoError(tb, encoder.Unmarshal(data, &payload))

	var declared core.DeclaredClassDefinition
	require.NoError(tb, declared.UnmarshalBinary(payload))

	return &declared
}

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

func TestDeclaredClassUnmarshalCBOR(t *testing.T) {
	for _, fixture := range declaredClassFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := encoder.Marshal(fixture.declared)
			require.NoError(t, err)

			var declared core.DeclaredClassDefinition
			require.NoError(t, declared.UnmarshalCBOR(data))

			assert.Equal(t, fixture.declared, &declared)
			assert.Equal(t, decodeDeclaredClassGeneric(t, data), &declared)
		})
	}
}

// TestDeclaredClassUnmarshalCBORSyntheticClasses covers the fields the feeder
// fixtures leave empty, so every branch of the decoder sees a value.
func TestDeclaredClassUnmarshalCBORSyntheticClasses(t *testing.T) {
	one := felt.FromUint64[felt.Felt](1)
	two := felt.FromUint64[felt.Felt](2)

	tests := []struct {
		name  string
		class core.ClassDefinition
	}{
		{
			name:  "empty sierra",
			class: &core.SierraClass{},
		},
		{
			name:  "empty deprecated cairo",
			class: &core.DeprecatedCairoClass{},
		},
		{
			name: "sierra with every field set",
			class: &core.SierraClass{
				Abi:     "abi",
				AbiHash: &one,
				EntryPoints: core.SierraEntryPointsByType{
					Constructor: []core.SierraEntryPoint{{Index: 0, Selector: &one}},
					External:    []core.SierraEntryPoint{{Index: 1, Selector: &two}},
					L1Handler:   []core.SierraEntryPoint{},
				},
				Program:         felt.Slice[felt.Felt]{one, two},
				ProgramHash:     &two,
				SemanticVersion: "1.0.0",
				Compiled: &core.CasmClass{
					Bytecode:        felt.Slice[felt.Felt]{one, two},
					PythonicHints:   []byte(`{"0":["hint"]}`),
					CompilerVersion: "2.1.0",
					Hints:           []byte(`[[0,[{"AllocSegment":{}}]]]`),
					Prime:           big.NewInt(7),
					External:        []core.CasmEntryPoint{{Offset: 3, Builtins: []string{"range_check"}, Selector: &one}},
					L1Handler:       []core.CasmEntryPoint{},
					Constructor:     nil,
					BytecodeSegmentLengths: core.SegmentLengths{
						Length: 4,
						Children: []core.SegmentLengths{
							{Length: 1},
							{Length: 2, Children: []core.SegmentLengths{{Length: 3}}},
						},
					},
				},
			},
		},
		{
			name: "deprecated cairo with every field set",
			class: &core.DeprecatedCairoClass{
				Abi:          []byte(`[{"type":"function"}]`),
				Externals:    []core.DeprecatedEntryPoint{{Selector: &one, Offset: &two}},
				L1Handlers:   []core.DeprecatedEntryPoint{},
				Constructors: []core.DeprecatedEntryPoint{{Selector: &two, Offset: &one}},
				Program:      "program",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := &core.DeclaredClassDefinition{At: 99, Class: test.class}

			data, err := encoder.Marshal(input)
			require.NoError(t, err)

			var declared core.DeclaredClassDefinition
			require.NoError(t, declared.UnmarshalCBOR(data))

			assert.Equal(t, input, &declared)
			assert.Equal(t, decodeDeclaredClassGeneric(t, data), &declared)
		})
	}
}

func TestDeclaredClassUnmarshalCBORRejectsBadInput(t *testing.T) {
	one := felt.FromUint64[felt.Felt](1)
	valid, err := encoder.Marshal(&core.DeclaredClassDefinition{
		At:    1,
		Class: &core.SierraClass{AbiHash: &one},
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{}},
		{name: "truncated", data: valid[:len(valid)/2]},
		{name: "not a byte string", data: []byte{0x63, 'a', 'b', 'c'}},
		{name: "payload shorter than the block number", data: []byte{0x43, 0x01, 0x02, 0x03}},
		{name: "unknown class tag", data: []byte{0x4a, 0, 0, 0, 0, 0, 0, 0, 1, 0xda, 0x00}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var declared core.DeclaredClassDefinition
			assert.Error(t, declared.UnmarshalCBOR(test.data))
		})
	}
}

// TestDeclaredClassCBORTags pins the tags the encoder registry assigns to the
// class types, which the fast path dispatches on.
func TestDeclaredClassCBORTags(t *testing.T) {
	tests := []struct {
		name  string
		class core.ClassDefinition
		tag   []byte
	}{
		{
			name:  "sierra",
			class: &core.SierraClass{},
			tag:   []byte{0xda, 0x00, 0x01, 0x00, 0x06},
		},
		{
			name:  "deprecatedCairo",
			class: &core.DeprecatedCairoClass{},
			tag:   []byte{0xda, 0x00, 0x01, 0x00, 0x05},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := encoder.Marshal(test.class)
			require.NoError(t, err)
			assert.Equal(t, test.tag, data[:len(test.tag)])
		})
	}
}

// BenchmarkDeclaredClassUnmarshalCBOR compares the hand-written decoder used on
// the getClass read path against the reflection based one it replaces, for both
// on-disk shapes. The map numbers are the ones that matter, since that is what a
// synced node reads (see unmarshalCBORFast).
//
// Each shape is measured against the generic decode the fallback would really do
// for it. They are not interchangeable: unwrapping the byte string copies the
// whole payload, which the map shape never pays, so benchmarking the fast path
// against the byte-string baseline alone overstates both the time and the memory
// it saves on real data.
func BenchmarkDeclaredClassUnmarshalCBOR(b *testing.B) {
	type plain core.DeclaredClassDefinition

	genericByteString := func(data []byte, out *core.DeclaredClassDefinition) error {
		var payload []byte
		if err := encoder.Unmarshal(data, &payload); err != nil {
			return err
		}
		return out.UnmarshalBinary(payload)
	}

	genericMap := func(data []byte, out *core.DeclaredClassDefinition) error {
		return encoder.Unmarshal(data, (*plain)(out))
	}

	for _, fixture := range declaredClassFixtures(b) {
		byteString, err := encoder.Marshal(fixture.declared)
		require.NoError(b, err)
		require.Equal(b, byte(2), byteString[0]>>5, "want a CBOR byte string (major 2)")

		mapShape, err := encoder.Marshal((*plain)(fixture.declared))
		require.NoError(b, err)
		require.Equal(b, byte(5), mapShape[0]>>5, "want a CBOR map (major 5)")

		shapes := []struct {
			name    string
			data    []byte
			generic func([]byte, *core.DeclaredClassDefinition) error
		}{
			{name: "byteString", data: byteString, generic: genericByteString},
			{name: "map", data: mapShape, generic: genericMap},
		}

		for _, shape := range shapes {
			b.Run(fixture.name+"/"+shape.name+"/fastPath", func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					var declared core.DeclaredClassDefinition
					if err := declared.UnmarshalCBOR(shape.data); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run(fixture.name+"/"+shape.name+"/generic", func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					var declared core.DeclaredClassDefinition
					if err := shape.generic(shape.data, &declared); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// TestDeclaredClassStoredShapeIsAByteString pins the shape the fast path decodes.
// WriteClass marshals through MarshalBinary, so the stored value is a byte string
// of the block number followed by the class. If the encoder ever stores a struct
// map instead, every read silently falls back to the reflection decoder.
func TestDeclaredClassStoredShapeIsAByteString(t *testing.T) {
	classHash := felt.FromUint64[felt.Felt](7)

	for _, fixture := range declaredClassFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) {
			memDB := memory.New()
			require.NoError(t, core.WriteClass(memDB, &classHash, fixture.declared))

			require.NoError(t, memDB.Get(db.ClassKey(&classHash), func(stored []byte) error {
				require.NotEmpty(t, stored)
				assert.Equal(t, byte(2), stored[0]>>5, "expected a CBOR byte string (major type 2)")
				return nil
			}))

			got, err := core.GetClass(memDB, &classHash)
			require.NoError(t, err)
			assert.Equal(t, fixture.declared, got)
		})
	}
}

// TestDeclaredClassUnmarshalCBORMapShape covers the shape a real node actually
// stores: the self-describing CBOR struct map {At, Class}, written by builds that
// predate DeclaredClassDefinition.MarshalBinary. TestDeclaredClassStoredShapeIsAByteString
// only proves what the current encoder writes; this proves the fast path decodes
// what is already on disk. Without the map fast path these values fall through to
// the reflection decoder and the hand-written decoder is dead code in production.
func TestDeclaredClassUnmarshalCBORMapShape(t *testing.T) {
	// Reproduce the on-disk shape: encoding through an alias skips MarshalBinary
	// and yields the {At, Class} map instead of the byte-string wrapper.
	type plain core.DeclaredClassDefinition

	for _, fixture := range declaredClassFixtures(t) {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := encoder.Marshal((*plain)(fixture.declared))
			require.NoError(t, err)
			require.NotEmpty(t, data)
			require.Equalf(t, byte(5), data[0]>>5, "want a CBOR map (major 5), got major %d", data[0]>>5)

			var got core.DeclaredClassDefinition
			require.NoError(t, got.UnmarshalCBOR(data))
			require.Equal(t, fixture.declared, &got)
		})
	}
}
