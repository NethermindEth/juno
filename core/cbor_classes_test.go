package core

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
)

type plannerClassDefinition ClassDefinition

func classTags(t testing.TB) CBORClassTags {
	t.Helper()
	tag := func(v any) uint64 {
		data, err := cbor.Marshal(v)
		require.NoError(t, err)
		n, err := fastcbor.TagOf(data)
		require.NoError(t, err)
		return n
	}
	return CBORClassTags{DeprecatedCairoClass: tag(DeprecatedCairoClass{}), SierraClass: tag(SierraClass{})}
}

func checkClassPlanner(t *testing.T, data []byte, tags CBORClassTags) {
	t.Helper()
	checkProjectionPlanner(t, data, func(v plannerClassDefinition) ClassDefinition { return ClassDefinition(v) })
	var direct, routed ClassDefinition
	rest, err := DecodeClassDefinitionFirst(data, &direct, tags)
	wantRest, wantErr := fastcbor.UnmarshalFirst(data, &routed)
	require.Equal(t, wantErr, err)
	require.Equal(t, wantRest, rest)
	require.Equal(t, routed, direct)
}

func randomSegments(rng *rand.Rand, depth int) SegmentLengths {
	out := SegmentLengths{Length: rng.Uint64()}
	if depth > 0 && rng.IntN(3) != 0 {
		out.Children = make([]SegmentLengths, rng.IntN(4))
		for i := range out.Children {
			out.Children[i] = randomSegments(rng, depth-1)
		}
	}
	return out
}

func randomClasses(rng *rand.Rand) []ClassDefinition {
	f := func() *felt.Felt {
		if rng.IntN(3) == 0 {
			return nil
		}
		return felt.NewFromUint64[felt.Felt](rng.Uint64())
	}
	raw := func() json.RawMessage {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make(json.RawMessage, rng.IntN(128))
		for i := range out {
			out[i] = byte(rng.Uint32())
		}
		return out
	}
	felts := func() felt.Slice[felt.Felt] {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make(felt.Slice[felt.Felt], rng.IntN(12))
		for i := range out {
			out[i] = felt.FromUint64[felt.Felt](rng.Uint64())
		}
		return out
	}
	deprecatedEntries := func() []DeprecatedEntryPoint {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make([]DeprecatedEntryPoint, rng.IntN(8))
		for i := range out {
			out[i] = DeprecatedEntryPoint{Selector: f(), Offset: f()}
		}
		return out
	}
	sierraEntries := func() []SierraEntryPoint {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make([]SierraEntryPoint, rng.IntN(8))
		for i := range out {
			out[i] = SierraEntryPoint{Index: rng.Uint64(), Selector: f()}
		}
		return out
	}
	casmEntries := func() []CasmEntryPoint {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make([]CasmEntryPoint, rng.IntN(8))
		for i := range out {
			out[i] = CasmEntryPoint{Offset: rng.Uint64(), Selector: f()}
			if rng.IntN(3) != 0 {
				out[i].Builtins = make([]string, rng.IntN(6))
				for j := range out[i].Builtins {
					out[i].Builtins[j] = []string{"", "pedersen", "range_check", "αβ🌍"}[rng.IntN(4)]
				}
			}
		}
		return out
	}
	cairo := &DeprecatedCairoClass{Abi: raw(), Program: "compressed-program αβ", Externals: deprecatedEntries(), L1Handlers: deprecatedEntries(), Constructors: deprecatedEntries()}
	sierra := &SierraClass{Abi: "[opaque ABI αβ]", AbiHash: f(), Program: felts(), ProgramHash: f(), SemanticVersion: "1.2.3", EntryPoints: SierraEntryPointsByType{Constructor: sierraEntries(), External: sierraEntries(), L1Handler: sierraEntries()}}
	if rng.IntN(3) != 0 {
		compiled := &CasmClass{Bytecode: felts(), PythonicHints: raw(), CompilerVersion: "2.3.α", Hints: raw(), External: casmEntries(), L1Handler: casmEntries(), Constructor: casmEntries(), BytecodeSegmentLengths: randomSegments(rng, 3)}
		if rng.IntN(3) != 0 {
			magnitude := make([]byte, rng.IntN(129))
			for i := range magnitude {
				magnitude[i] = byte(rng.Uint32())
			}
			compiled.Prime = new(big.Int).SetBytes(magnitude)
			if rng.IntN(2) == 0 {
				compiled.Prime.Neg(compiled.Prime)
			}
		}
		sierra.Compiled = compiled
	}
	return []ClassDefinition{cairo, sierra}
}

func TestTypedClasses(t *testing.T) {
	tags := classTags(t)
	rng := rand.New(rand.NewPCG(441, 942))
	for range 128 {
		for _, value := range randomClasses(rng) {
			data, err := cbor.Marshal(value)
			require.NoError(t, err)
			checkClassPlanner(t, data, tags)
			checkReceiptOwned(t, data, value)
			var direct ClassDefinition
			require.NoError(t, DecodeClassDefinition(data, &direct, tags))
			require.Equal(t, value, direct)
		}
	}
	for _, data := range [][]byte{{0xf6}, {0xf7}} {
		checkClassPlanner(t, data, tags)
		checkReceiptOwned[ClassDefinition](t, data, nil)
	}
	for _, value := range []ClassDefinition{
		&DeprecatedCairoClass{Abi: json.RawMessage{}, Externals: []DeprecatedEntryPoint{}, L1Handlers: []DeprecatedEntryPoint{}, Constructors: []DeprecatedEntryPoint{}},
		&SierraClass{Program: felt.Slice[felt.Felt]{}, EntryPoints: SierraEntryPointsByType{Constructor: []SierraEntryPoint{}, External: []SierraEntryPoint{}, L1Handler: []SierraEntryPoint{}}, Compiled: &CasmClass{Prime: new(big.Int), Bytecode: felt.Slice[felt.Felt]{}, PythonicHints: json.RawMessage{}, Hints: json.RawMessage{}, External: []CasmEntryPoint{}, L1Handler: []CasmEntryPoint{}, Constructor: []CasmEntryPoint{}, BytecodeSegmentLengths: SegmentLengths{Children: []SegmentLengths{}}}},
	} {
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		checkClassPlanner(t, data, tags)
		checkReceiptOwned(t, data, value)
	}
}

func TestTypedClassFailures(t *testing.T) {
	tags := classTags(t)
	for _, value := range randomClasses(rand.New(rand.NewPCG(5, 821))) {
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		for end := range len(data) {
			checkClassPlanner(t, data[:end], tags)
			var got ClassDefinition
			rest, err := DecodeClassDefinitionFirst(data[:end], &got, tags)
			require.Error(t, err)
			require.Nil(t, rest)
			require.Nil(t, got)
		}
		got := value
		rest, err := DecodeClassDefinitionFirst([]byte{0xf6}, &got, tags)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Same(t, value, got)
		var trailing ClassDefinition
		require.ErrorIs(t, DecodeClassDefinition(append(bytes.Clone(data), 7), &trailing, tags), fastcbor.ErrExtraneousData)
		require.Nil(t, trailing)
	}
	for _, value := range []ClassDefinition{(*DeprecatedCairoClass)(nil), (*SierraClass)(nil)} {
		got := value
		rest, err := DecodeClassDefinitionFirst([]byte{0xf6}, &got, tags)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, value, got)
	}
}

func classCompiledWire(t *testing.T, tags CBORClassTags, key string, value []byte) []byte {
	t.Helper()
	compiled := projectionMap(t, projectionPair{key, value})
	return append(transactionTag(tags.SierraClass, false), projectionMap(t, projectionPair{"Compiled", compiled})...)
}

func TestTypedClassPrime(t *testing.T) {
	tags := classTags(t)
	for _, body := range [][]byte{
		{0}, {0x17}, {0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, {0x20}, {0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0xc2, 0x40}, {0xc3, 0x40}, {0xc2, 0x43, 0, 0, 1}, {0xc3, 0x43, 0, 0, 1}, {0xc2, 0x49, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{0xf6}, {0xf7}, {0xc2, 0x61, 'x'}, {0xc3, 0x49, 1}, {0xc4, 0x40}, {0x81, 1}, {0xc2, 0xc2, 0x40},
	} {
		wire := classCompiledWire(t, tags, "Prime", body)
		checkClassPlanner(t, wire, tags)
		var got ClassDefinition
		if err := DecodeClassDefinition(wire, &got, tags); err == nil {
			checkReceiptOwned(t, wire, got)
		} else {
			require.Nil(t, got)
		}
	}
}

func TestTypedClassRecursiveSegments(t *testing.T) {
	tags := classTags(t)
	for _, links := range []int{0, 1, 13, 14, 15, 16} {
		for _, leaf := range [][]byte{
			projectionMap(t, projectionPair{"Length", []byte{7}}),
			projectionMap(t, projectionPair{"Children", []byte{0x81, 0xf6}}),
			projectionMap(t, projectionPair{"Children", []byte{0x81, 0xa0}}),
		} {
			node := leaf
			for range links {
				node = projectionMap(t, projectionPair{"Children", append([]byte{0x81}, node...)})
			}
			wire := classCompiledWire(t, tags, "BytecodeSegmentLengths", node)
			checkClassPlanner(t, wire, tags)
			var got ClassDefinition
			err := DecodeClassDefinition(wire, &got, tags)
			extraMap := leaf[len(leaf)-1] == 0xa0
			if links < 14 || links == 14 && !extraMap {
				require.NoError(t, err)
				checkReceiptOwned(t, wire, got)
			} else {
				require.ErrorIs(t, err, cursor.ErrDepth)
				require.Nil(t, got)
			}
		}
	}
	// A null record in a value slice leaves a zero element without adding a map level.
	for _, body := range [][]byte{{0xf6}, {0xf7}, {0xa0}, projectionMap(t, projectionPair{"Children", []byte{0x83, 0xf6, 0xa0, 0xf7}})} {
		wire := classCompiledWire(t, tags, "BytecodeSegmentLengths", body)
		checkClassPlanner(t, wire, tags)
		var got ClassDefinition
		require.NoError(t, DecodeClassDefinition(wire, &got, tags))
		checkReceiptOwned(t, wire, got)
	}
}

func TestTypedClassWireCorners(t *testing.T) {
	tags := classTags(t)
	for _, tag := range []uint64{tags.DeprecatedCairoClass, tags.SierraClass} {
		for _, wide := range []bool{false, true} {
			for _, body := range [][]byte{{0xa0}, {0xf6}, {0xf7}, {0x80}, projectionMap(t, projectionPair{"Abi", []byte{0xf6}}, projectionPair{"ABI", []byte{0xf6}})} {
				checkClassPlanner(t, append(transactionTag(tag, wide), body...), tags)
			}
		}
	}
	for _, tag := range []uint64{0, 1, 2, 3, 4, 24, 255, 256, 65535, 65536, math.MaxUint64} {
		wire := append(transactionTag(tag, false), 0xa0)
		checkClassPlanner(t, wire, tags)
		checkClassPlanner(t, append(transactionTag(tags.SierraClass, false), wire...), tags)
	}
	for _, field := range []string{"Bytecode", "PythonicHints", "CompilerVersion", "Hints", "External", "L1Handler", "Constructor", "BytecodeSegmentLengths"} {
		for _, body := range [][]byte{{0xf6}, {0xf7}, {0x80}, {0xa0}, {0x61, 0xff}, {0x43, 0xff, 0, 0xab}, {0x81, 0xf6}, {0x81, 0xa0}, {0x9f, 0xff}, {0xbf, 0xff}, {0xff}} {
			checkClassPlanner(t, classCompiledWire(t, tags, field, body), tags)
		}
	}
	// Raw JSON fields are opaque bytes, including bytes which are not valid JSON or UTF-8.
	wire := append(transactionTag(tags.DeprecatedCairoClass, false), projectionMap(t, projectionPair{"aBi", []byte{0x43, 0xff, 0, 0xab}})...)
	checkReceiptOwned[ClassDefinition](t, wire, &DeprecatedCairoClass{Abi: json.RawMessage{0xff, 0, 0xab}})
	// Null strings inside the builtin array become empty Go strings.
	ep := projectionMap(t, projectionPair{"Builtins", []byte{0x84, 0xf6, 0xf7, 0x60, 0x61, 'x'}})
	wire = classCompiledWire(t, tags, "External", append([]byte{0x81}, ep...))
	checkClassPlanner(t, wire, tags)
	var got ClassDefinition
	require.NoError(t, DecodeClassDefinition(wire, &got, tags))
	require.Equal(t, []string{"", "", "", "x"}, got.(*SierraClass).Compiled.External[0].Builtins)
	checkReceiptOwned(t, wire, got)
	// Late failure after allocating a compiled object, entry slice and strings.
	compiled := projectionMap(t, projectionPair{"External", append([]byte{0x81}, ep...)}, projectionPair{"Hints", []byte{0xff}})
	wire = append(transactionTag(tags.SierraClass, false), projectionMap(t, projectionPair{"Compiled", compiled})...)
	checkClassPlanner(t, wire, tags)
	got = nil
	require.Error(t, DecodeClassDefinition(wire, &got, tags))
	require.Nil(t, got)
}

func TestTypedClassTagProfile(t *testing.T) {
	tags := CBORClassTags{DeprecatedCairoClass: 1000000, SierraClass: math.MaxUint64}
	for _, tc := range []struct {
		tag  uint64
		want ClassDefinition
	}{{tags.DeprecatedCairoClass, &DeprecatedCairoClass{}}, {tags.SierraClass, &SierraClass{}}} {
		input := append(transactionTag(tc.tag, false), 0xa0, 7)
		var got ClassDefinition
		rest, err := DecodeClassDefinitionFirst(input, &got, tags)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
		require.Equal(t, []byte{7}, rest)
	}
	require.PanicsWithValue(t, "core: duplicate typed class tags", func() { RegisterTypedClassReader(CBORClassTags{}) })
}

func FuzzTypedClasses(f *testing.F) {
	tags := classTags(f)
	for _, value := range randomClasses(rand.New(rand.NewPCG(133, 28))) {
		data, err := cbor.Marshal(value)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	for _, seed := range [][]byte{{0xa0}, {0xf6}, {0xf7}, append(transactionTag(tags.SierraClass, false), 0xa0)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkClassPlanner(t, data, tags) })
}
