package statedifflength

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/require"
)

// randomFelt returns a felt whose limbs cover every CBOR uint width, so the walker
// is exercised against short and long felt encodings alike.
func randomFelt(source *rand.Rand) *felt.Felt {
	switch source.IntN(4) {
	case 0:
		return felt.NewFromUint64[felt.Felt](uint64(source.IntN(24))) // single-byte limbs
	case 1:
		return felt.NewFromUint64[felt.Felt](uint64(source.IntN(1 << 16)))
	case 2:
		return felt.NewFromUint64[felt.Felt](source.Uint64())
	default:
		var bytes [32]byte
		for i := range 4 {
			binary.BigEndian.PutUint64(bytes[i*8:], source.Uint64())
		}
		return felt.NewFromBytes[felt.Felt](bytes[:])
	}
}

// randomSize picks a collection size that spans the CBOR count widths: -1 for a nil
// collection (encoded as null), 0 for an empty one, then inline counts (< 24) and
// one-byte counts (>= 24) up to scale.
func randomSize(source *rand.Rand, scale int) int {
	switch source.IntN(4) {
	case 0:
		return -1
	case 1:
		return 0
	default:
		return source.IntN(scale + 1)
	}
}

// randomFeltMap returns nil for a negative size, so the encoder writes null.
func randomFeltMap(source *rand.Rand, size int) map[felt.Felt]*felt.Felt {
	if size < 0 {
		return nil
	}
	entries := make(map[felt.Felt]*felt.Felt, size)
	for range size {
		entries[*randomFelt(source)] = randomFelt(source)
	}
	return entries
}

func randomStateDiff(source *rand.Rand, scale int) *core.StateDiff {
	diff := &core.StateDiff{
		Nonces:            randomFeltMap(source, randomSize(source, scale)),
		DeployedContracts: randomFeltMap(source, randomSize(source, scale)),
		DeclaredV1Classes: randomFeltMap(source, randomSize(source, scale)),
		ReplacedClasses:   randomFeltMap(source, randomSize(source, scale)),
	}

	if contracts := randomSize(source, scale); contracts >= 0 {
		diff.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt, contracts)
		for range contracts {
			// An empty inner map is legal and contributes nothing to the length.
			diff.StorageDiffs[*randomFelt(source)] = randomFeltMap(source, source.IntN(scale+1))
		}
	}
	if declaredV0 := randomSize(source, scale); declaredV0 >= 0 {
		diff.DeclaredV0Classes = make([]*felt.Felt, 0, declaredV0)
		for range declaredV0 {
			diff.DeclaredV0Classes = append(diff.DeclaredV0Classes, randomFelt(source))
		}
	}
	if migrated := randomSize(source, scale); migrated >= 0 {
		diff.MigratedClasses = make(map[felt.SierraClassHash]felt.CasmClassHash, migrated)
		for range migrated {
			diff.MigratedClasses[felt.SierraClassHash(*randomFelt(source))] = felt.CasmClassHash(*randomFelt(source))
		}
	}
	return diff
}

// randomStateUpdate varies the fields the walker skips past on its way to the state
// diff. Leaving them nil half the time covers null, a different CBOR shape to the
// four-element array a felt encodes to; the store path always fills the roots in, so
// this is shape coverage rather than a record the migration will meet.
func randomStateUpdate(source *rand.Rand, scale int) *core.StateUpdate {
	stateUpdate := &core.StateUpdate{StateDiff: randomStateDiff(source, scale)}
	if source.IntN(2) == 0 {
		stateUpdate.BlockHash = randomFelt(source)
		stateUpdate.NewRoot = randomFelt(source)
		stateUpdate.OldRoot = randomFelt(source)
	}
	return stateUpdate
}

// TestStateDiffLengthMatchesDecoder is the equivalence proof: for the same stored
// bytes, the walker must return exactly what decoding and calling
// core.StateDiff.Length() returns. It fails if a counted core.StateDiff field is
// renamed, added or dropped, because the new field would go uncounted.
func TestStateDiffLengthMatchesDecoder(t *testing.T) {
	// A scale of 30 crosses the inline CBOR count boundary at 24 in both the outer
	// and inner collections.
	for _, scale := range []int{0, 1, 5, 30} {
		t.Run(fmt.Sprintf("scale=%d", scale), func(t *testing.T) {
			source := rand.New(rand.NewPCG(uint64(scale), 0x5eed))
			for iteration := range 200 {
				stateUpdate := randomStateUpdate(source, scale)

				data, err := encoder.Marshal(stateUpdate)
				require.NoError(t, err)

				// Decode the bytes back rather than trusting the in-memory value, so
				// the comparison is against exactly what the migration used to do.
				var decoded *core.StateUpdate
				require.NoError(t, encoder.Unmarshal(data, &decoded))

				walked, err := stateDiffLength(data)
				require.NoError(t, err, "iteration %d", iteration)
				require.Equal(t, decoded.StateDiff.Length(), walked, "iteration %d", iteration)
			}
		})
	}
}

// TestStateDiffLengthCountsEveryField guards against a field being counted twice or
// not at all, which random diffs could mask by coincidence.
func TestStateDiffLengthCountsEveryField(t *testing.T) {
	one := felt.NewFromUint64[felt.Felt](1)
	tests := map[string]struct {
		diff   *core.StateDiff
		length uint64
	}{
		"empty": {diff: &core.StateDiff{}, length: 0},
		"storage diffs": {diff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*one:                          {*one: one, felt.FromUint64[felt.Felt](2): one},
				felt.FromUint64[felt.Felt](3): {*one: one},
				felt.FromUint64[felt.Felt](4): {}, // empty inner map counts as zero
			},
		}, length: 3},
		"nonces": {diff: &core.StateDiff{
			Nonces: map[felt.Felt]*felt.Felt{*one: one},
		}, length: 1},
		"deployed contracts": {diff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{*one: one},
		}, length: 1},
		"declared v0 classes": {diff: &core.StateDiff{
			DeclaredV0Classes: []*felt.Felt{one, one, one},
		}, length: 3},
		"declared v1 classes": {diff: &core.StateDiff{
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{*one: one},
		}, length: 1},
		"replaced classes": {diff: &core.StateDiff{
			ReplacedClasses: map[felt.Felt]*felt.Felt{*one: one},
		}, length: 1},
		"migrated classes": {diff: &core.StateDiff{
			MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{
				felt.SierraClassHash(*one): felt.CasmClassHash(*one),
			},
		}, length: 1},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := encoder.Marshal(&core.StateUpdate{StateDiff: test.diff})
			require.NoError(t, err)

			length, err := stateDiffLength(data)
			require.NoError(t, err)
			require.Equal(t, test.length, length)
			require.Equal(t, test.diff.Length(), length)
		})
	}
}

// TestStateDiffLengthMatchesDecoderFieldNaming pins the walker to the decoder's key
// resolution: an exact match, or a case-insensitive fallback of the same byte
// length. The encoder only ever writes exact names, so this covers foreign or
// corrupt records rather than anything juno wrote.
func TestStateDiffLengthMatchesDecoderFieldNaming(t *testing.T) {
	one := felt.NewFromUint64[felt.Felt](1)
	nonces := map[felt.Felt]*felt.Felt{*one: one}

	tests := map[string]struct {
		stateDiffKey string
		noncesKey    string
		length       uint64
	}{
		"exact":                {stateDiffKey: "StateDiff", noncesKey: "Nonces", length: 1},
		"folded field":         {stateDiffKey: "StateDiff", noncesKey: "NonCes", length: 1},
		"folded parent":        {stateDiffKey: "statediff", noncesKey: "nonces", length: 1},
		"different length":     {stateDiffKey: "StateDiff", noncesKey: "Nonce", length: 0},
		"unknown field":        {stateDiffKey: "StateDiff", noncesKey: "Noncess", length: 0},
		"non-ascii same bytes": {stateDiffKey: "StateDiff", noncesKey: "Noncеs", length: 0},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := encoder.Marshal(map[string]any{
				test.stateDiffKey: map[string]any{test.noncesKey: nonces},
			})
			require.NoError(t, err)

			// The decoder is the reference: whatever it makes of these keys, the
			// walker must agree.
			var decoded *core.StateUpdate
			require.NoError(t, encoder.Unmarshal(data, &decoded))
			require.Equal(t, test.length, decoded.StateDiff.Length(), "decoder reference")

			walked, err := stateDiffLength(data)
			require.NoError(t, err)
			require.Equal(t, test.length, walked)
		})
	}
}

func TestStateDiffLengthRejectsBadInput(t *testing.T) {
	marshal := func(t *testing.T, value any) []byte {
		t.Helper()
		data, err := encoder.Marshal(value)
		require.NoError(t, err)
		return data
	}

	t.Run("null state update", func(t *testing.T) {
		_, err := stateDiffLength(marshal(t, (*core.StateUpdate)(nil)))
		require.ErrorIs(t, err, errNilStateUpdate)
	})

	t.Run("null state diff", func(t *testing.T) {
		_, err := stateDiffLength(marshal(t, &core.StateUpdate{}))
		require.ErrorIs(t, err, errNilStateDiff)
	})

	t.Run("no state diff field", func(t *testing.T) {
		_, err := stateDiffLength(marshal(t, struct{ BlockHash *felt.Felt }{}))
		require.ErrorIs(t, err, errNoStateDiff)
	})

	t.Run("not a map", func(t *testing.T) {
		_, err := stateDiffLength(marshal(t, []uint64{1, 2, 3}))
		require.ErrorIs(t, err, errNotACollection)
	})

	t.Run("counted field is not a collection", func(t *testing.T) {
		_, err := stateDiffLength(marshal(t, map[string]any{
			keyStateDiff: map[string]any{keyNonces: "not a map"},
		}))
		require.ErrorIs(t, err, errNotACollection)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := stateDiffLength(nil)
		require.Error(t, err)
	})

	t.Run("truncated", func(t *testing.T) {
		source := rand.New(rand.NewPCG(1, 2))
		data := marshal(t, randomStateUpdate(source, 8))
		for cut := 1; cut < len(data); cut++ {
			_, err := stateDiffLength(data[:len(data)-cut])
			require.Error(t, err, "truncating %d bytes must not be accepted", cut)
		}
	})
}

// TestStateDiffLengthDoesNotAllocate is the other half of the claim: the walk is
// allocation-free, so per-block cost no longer scales with the size of the diff.
func TestStateDiffLengthDoesNotAllocate(t *testing.T) {
	source := rand.New(rand.NewPCG(7, 7))
	data, err := encoder.Marshal(randomStateUpdate(source, 40))
	require.NoError(t, err)

	length, err := stateDiffLength(data) // warm up, then measure
	require.NoError(t, err)
	require.NotZero(t, length)

	allocations := testing.AllocsPerRun(100, func() {
		if _, err := stateDiffLength(data); err != nil {
			t.Fatal(err)
		}
	})
	require.Zero(t, allocations)
}

// FuzzStateDiffLength checks that arbitrary bytes are rejected rather than crashing
// the migration, and that anything accepted agrees with the decoder.
//
// The seeds matter more than the run length. A mutation only reaches the comparison if
// both the walker and the decoder accept it, so seeding structurally varied records —
// every counted field populated, nil and empty collections, hand-built shapes — is what
// keeps the mutator producing inputs that test agreement rather than just rejection.
func FuzzStateDiffLength(f *testing.F) {
	source := rand.New(rand.NewPCG(3, 4))
	for _, scale := range []int{0, 1, 2, 25, 30} {
		data, err := encoder.Marshal(randomStateUpdate(source, scale))
		require.NoError(f, err)
		f.Add(data)
	}

	// Every counted field non-empty at once, which the random scales only hit by chance.
	one := felt.NewFromUint64[felt.Felt](1)
	full, err := encoder.Marshal(&core.StateUpdate{
		BlockHash: one, NewRoot: one, OldRoot: one,
		StateDiff: &core.StateDiff{
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*one: {*one: one, felt.FromUint64[felt.Felt](2): one},
			},
			Nonces:            map[felt.Felt]*felt.Felt{*one: one},
			DeployedContracts: map[felt.Felt]*felt.Felt{*one: one},
			DeclaredV0Classes: []*felt.Felt{one, one},
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{*one: one},
			ReplacedClasses:   map[felt.Felt]*felt.Felt{*one: one},
			MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{
				felt.SierraClassHash(*one): felt.CasmClassHash(*one),
			},
		},
	})
	require.NoError(f, err)
	f.Add(full)

	// Hand-built shapes the encoder cannot produce, to give the mutator structural
	// starting points as well as realistic ones.
	f.Add(stateUpdateWith(stateDiffWith([]byte{0x0a}))) // an unknown field
	f.Add(stateUpdateWith([]byte{0xa0}))                // empty state diff
	f.Add(stateUpdateWith([]byte{cborNull}))            // null state diff
	f.Add([]byte{0xa0})                                 // state update with no fields
	f.Add([]byte{cborNull})                             // null state update

	f.Fuzz(func(t *testing.T, data []byte) {
		walked, err := stateDiffLength(data)
		if err != nil {
			return
		}

		var decoded *core.StateUpdate
		if err := encoder.Unmarshal(data, &decoded); err != nil {
			return // the walker is more permissive about trailing bytes and types
		}
		if decoded == nil || decoded.StateDiff == nil {
			t.Fatalf("accepted a state update the decoder read as nil: %x", data)
		}
		require.Equal(t, decoded.StateDiff.Length(), walked)
	})
}

func BenchmarkStateDiffLength(b *testing.B) {
	for _, scale := range []int{4, 20, 60} {
		source := rand.New(rand.NewPCG(uint64(scale), 11))
		data, err := encoder.Marshal(randomStateUpdate(source, scale))
		require.NoError(b, err)

		name := fmt.Sprintf("scale=%d/bytes=%d", scale, len(data))
		b.Run(name+"/decode", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var decoded *core.StateUpdate
				if err := encoder.Unmarshal(data, &decoded); err != nil {
					b.Fatal(err)
				}
				_ = decoded.StateDiff.Length()
			}
		})
		b.Run(name+"/walk", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := stateDiffLength(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
