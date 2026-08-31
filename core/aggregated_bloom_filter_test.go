package core_test

import (
	"encoding/binary"
	"io"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

// insertKey inserts key into filter at block using a fresh single-key bloom.
func insertKey(tb testing.TB, filter *core.AggregatedBloomFilter, key []byte, block uint64) {
	tb.Helper()
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	b.Add(key)
	require.NoError(tb, filter.Insert(b, block))
}

// mustMarshal returns filter's binary encoding, failing the test on error.
func mustMarshal(tb testing.TB, filter *core.AggregatedBloomFilter) []byte {
	tb.Helper()
	data, err := filter.MarshalBinary()
	require.NoError(tb, err)
	return data
}

type queryFilter interface {
	UnmarshalBinary(data []byte) error
	BlocksForKeysInto(keys [][]byte, out *bitset.BitSet) error
	FromBlock() uint64
	ToBlock() uint64
}

var filterVariants = []struct {
	name     string
	lazyRows bool
	new      func() queryFilter
}{
	{name: "filter", new: func() queryFilter { return &core.AggregatedBloomFilter{} }},
	{
		name:     "view",
		lazyRows: true,
		new:      func() queryFilter { return &core.AggregatedBloomFilterView{} },
	},
}

func mustUnmarshal(t *testing.T, newFilter func() queryFilter, data []byte) queryFilter {
	t.Helper()
	decoded := newFilter()
	require.NoError(t, decoded.UnmarshalBinary(data))
	return decoded
}

func blocksForKeys(t *testing.T, f queryFilter, keys [][]byte) *bitset.BitSet {
	t.Helper()
	out := bitset.New(uint(core.NumBlocksPerFilter))
	require.NoError(t, f.BlocksForKeysInto(keys, out))
	return out
}

func TestAggregatedBloomFilter_Insert(t *testing.T) {
	filter := core.NewAggregatedFilter(200)
	b := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	key := []byte{0x01, 0x02}
	b.Add(key)

	t.Run("Insert at valid block in range", func(t *testing.T) {
		tests := []struct {
			description string
			blockNumber uint64
		}{
			{
				description: "first block in range",
				blockNumber: 200,
			},
			{
				description: "between (first, last) block in range",
				blockNumber: 201,
			},
			{
				description: "last block in range",
				blockNumber: 200 + core.MaxBlockOffsetPerFilter,
			},
		}

		for _, test := range tests {
			require.NoError(t, filter.Insert(b, test.blockNumber))
			matches := filter.BlocksForKeys([][]byte{key})
			relativeBlockNumber := test.blockNumber - filter.FromBlock()
			require.True(t, matches.Test(uint(relativeBlockNumber)))
		}
	})

	t.Run("Insert at out-of-range block", func(t *testing.T) {
		require.ErrorIs(t, filter.Insert(b, filter.FromBlock()-1), core.ErrAggregatedBloomFilterBlockOutOfRange)
		require.ErrorIs(t, filter.Insert(b, filter.ToBlock()+1), core.ErrAggregatedBloomFilterBlockOutOfRange)
	})

	t.Run("Insert with wrong-sized bloom", func(t *testing.T) {
		differentSizeBloom := bloom.New(core.EventsBloomLength-1, core.EventsBloomHashFuncs)

		err := filter.Insert(differentSizeBloom, 201)
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrBloomFilterSizeMismatch)
	})
}

func TestAggregatedBloomFilter_BlocksForKeys(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	key := []byte{0xab}
	insertKey(t, &filter, key, 0)

	t.Run("No keys: all bits set", func(t *testing.T) {
		matches := filter.BlocksForKeys(nil)
		require.True(t, matches.All())
	})

	t.Run("Unmatched key: returns none", func(t *testing.T) {
		matches := filter.BlocksForKeys([][]byte{{0xff}})
		require.False(t, matches.Any())
	})

	t.Run("Known key: block 0 is set", func(t *testing.T) {
		matches := filter.BlocksForKeys([][]byte{key})
		require.True(t, matches.Test(0))
	})
}

func TestAggregatedBloomFilter_BlocksForKeysInto(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	key := []byte{0xab}
	insertKey(t, &filter, key, 0)
	data := mustMarshal(t, &filter)

	for _, variant := range filterVariants {
		t.Run(variant.name, func(t *testing.T) {
			decoded := mustUnmarshal(t, variant.new, data)
			matchesBuf := bitset.New(uint(core.NumBlocksPerFilter))

			t.Run("No keys: all bits set", func(t *testing.T) {
				require.NoError(t, decoded.BlocksForKeysInto(nil, matchesBuf))
				require.True(t, matchesBuf.All())
			})

			t.Run("Unmatched key: returns none", func(t *testing.T) {
				require.NoError(t, decoded.BlocksForKeysInto([][]byte{{0xff}}, matchesBuf))
				require.False(t, matchesBuf.Any())
			})

			t.Run("Known key: block 0 is set", func(t *testing.T) {
				require.NoError(t, decoded.BlocksForKeysInto([][]byte{key}, matchesBuf))
				require.True(t, matchesBuf.Test(0))
			})

			t.Run("Buffer size mismatch", func(t *testing.T) {
				differentSizeBuf := bitset.New(uint(core.NumBlocksPerFilter - 1))
				require.ErrorIs(
					t,
					decoded.BlocksForKeysInto([][]byte{key}, differentSizeBuf),
					core.ErrMatchesBufferSizeMismatch,
				)
			})

			t.Run("Buffer is nil", func(t *testing.T) {
				require.ErrorIs(
					t,
					decoded.BlocksForKeysInto([][]byte{key}, nil),
					core.ErrMatchesBufferNil,
				)
			})
		})
	}
}

func TestAggregatedBloomFilter_Clone(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	key := []byte{0x77}
	insertKey(t, &filter, key, 0)
	cp := filter.Clone()
	require.NotSame(t, &filter, &cp)
	require.Equal(t, filter, cp)
	// Mutate copy: shouldn't change origin
	insertKey(t, &cp, key, 1)
	require.True(t, cp.BlocksForKeys([][]byte{key}).Test(1))
	require.False(t, filter.BlocksForKeys([][]byte{key}).Test(1))
}

func TestAggregatedBloomFilter_Serialise(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	insertKey(t, &filter, []byte{0x33}, 0)

	data := mustMarshal(t, &filter)

	filter2 := &core.AggregatedBloomFilter{}
	require.NoError(t, filter2.UnmarshalBinary(data))
	require.Equal(t, filter, *filter2)
}

func TestAggregatedBloomFilter_UnmarshalBinary_Compat(t *testing.T) {
	// Build a filter with a couple of keys at known blocks.
	filter := core.NewAggregatedFilter(0)
	keyA := []byte{0x33}
	keyB := []byte{0xAB, 0xCD}
	insertKey(t, &filter, keyA, 0)
	insertKey(t, &filter, keyB, 5)

	data := mustMarshal(t, &filter)

	for _, variant := range filterVariants {
		t.Run(variant.name, func(t *testing.T) {
			// Matching behavior is preserved after decode.
			decoded := mustUnmarshal(t, variant.new, data)
			require.True(t, blocksForKeys(t, decoded, [][]byte{keyA}).Test(0))
			require.True(t, blocksForKeys(t, decoded, [][]byte{keyB}).Test(5))
			require.False(t, blocksForKeys(t, decoded, [][]byte{keyB}).Test(0))

			// Short/corrupt input returns a non-nil, non-panicking result.
			require.Error(t, variant.new().UnmarshalBinary(data[:10]))
			require.Error(t, variant.new().UnmarshalBinary(nil))
		})
	}
}

// Round-trips a filter whose range does not start at block 0, so a byte-offset
// bug in the header parse cannot hide behind fromBlock == 0.
func TestAggregatedBloomFilter_UnmarshalBinary_NonZeroRange(t *testing.T) {
	const from = 3 * core.NumBlocksPerFilter // distinctive, non-zero start
	filter := core.NewAggregatedFilter(from)
	key := []byte{0x5e}
	insertKey(t, &filter, key, from+7)

	data := mustMarshal(t, &filter)

	for _, variant := range filterVariants {
		t.Run(variant.name, func(t *testing.T) {
			decoded := mustUnmarshal(t, variant.new, data)
			require.Equal(t, from, decoded.FromBlock())
			require.Equal(t, from+core.MaxBlockOffsetPerFilter, decoded.ToBlock())
			require.True(t, blocksForKeys(t, decoded, [][]byte{key}).Test(7))
		})
	}
}

// Pins the on-disk header layout (fromBlock:uint64, toBlock:uint64,
// count:uint32, all big-endian) that UnmarshalBinary relies on, guarding
// against a future change to MarshalBinary silently breaking the wire format.
func TestAggregatedBloomFilter_MarshalBinary_HeaderLayout(t *testing.T) {
	const from = 0x0102030405060708
	filter := core.NewAggregatedFilter(from)

	data := mustMarshal(t, &filter)
	require.GreaterOrEqual(t, len(data), 20)

	require.Equal(t, uint64(from), binary.BigEndian.Uint64(data[0:8]))
	require.Equal(t, from+core.MaxBlockOffsetPerFilter, binary.BigEndian.Uint64(data[8:16]))
	require.Equal(t, uint32(core.EventsBloomLength), binary.BigEndian.Uint32(data[16:20]))
}

// Exercises the decode validation branches: framing corruption is rejected at
// decode; row corruption surfaces at decode (filter) or at query time (view).
func TestAggregatedBloomFilter_UnmarshalBinary_Corrupt(t *testing.T) {
	filter := core.NewAggregatedFilter(0)
	key := []byte{0x01}
	insertKey(t, &filter, key, 0)

	data := mustMarshal(t, &filter)

	// Row corruption must target a row the key hashes to, since the view reads
	// only the queried rows. Row i starts at header (20) + i*rowSize, with
	// rowSize = 4-byte blob length + 8-byte bit length + 1024 row bytes.
	keyRow := bloom.Locations(key, core.EventsBloomHashFuncs)[0] % core.EventsBloomLength
	rowSize := 4 + 8 + core.NumBlocksPerFilter/8
	rowStart := 20 + keyRow*rowSize

	corruptCopy := func(mutate func(corrupt []byte)) []byte {
		corrupt := append([]byte{}, data...)
		mutate(corrupt)
		return corrupt
	}

	tests := []struct {
		name    string
		corrupt []byte
		wantErr error
		lazyRow bool // for the view: error surfaces at query time, not decode
	}{
		{
			name: "wrong row bit-length",
			corrupt: corruptCopy(func(corrupt []byte) {
				binary.BigEndian.PutUint64(corrupt[rowStart+4:], core.NumBlocksPerFilter+1)
			}),
			wantErr: core.ErrBloomFilterSizeMismatch,
			lazyRow: true,
		},
		{
			// A non-canonical length that still fits in-bounds must be rejected.
			name: "wrong row blob-length",
			corrupt: corruptCopy(func(corrupt []byte) {
				binary.BigEndian.PutUint32(corrupt[rowStart:], 8)
			}),
			wantErr: core.ErrBloomFilterSizeMismatch,
			lazyRow: true,
		},
		{
			name:    "truncated header",
			corrupt: data[:10],
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "truncated mid-row",
			corrupt: data[:30],
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name: "wrong row count",
			corrupt: corruptCopy(func(corrupt []byte) {
				binary.BigEndian.PutUint32(corrupt[16:20], core.EventsBloomLength-1)
			}),
			wantErr: core.ErrBloomFilterSizeMismatch,
		},
		{
			name: "toBlock not matching range",
			corrupt: corruptCopy(func(corrupt []byte) {
				binary.BigEndian.PutUint64(corrupt[8:16], 1<<40)
			}),
			wantErr: core.ErrBloomFilterSizeMismatch,
		},
		{
			name:    "trailing bytes",
			corrupt: append(append([]byte{}, data...), 0x00, 0x01, 0x02, 0x03),
			wantErr: io.ErrUnexpectedEOF,
		},
	}

	for _, variant := range filterVariants {
		for _, test := range tests {
			t.Run(variant.name+"/"+test.name, func(t *testing.T) {
				decoded := variant.new()
				if test.lazyRow && variant.lazyRows {
					require.NoError(t, decoded.UnmarshalBinary(test.corrupt))
					out := bitset.New(uint(core.NumBlocksPerFilter))
					require.ErrorIs(t, decoded.BlocksForKeysInto([][]byte{key}, out), test.wantErr)
					return
				}
				require.ErrorIs(t, decoded.UnmarshalBinary(test.corrupt), test.wantErr)
			})
		}
	}
}

// UnmarshalBinary decodes untrusted DB bytes and must never panic on arbitrary
// input, only return an error.
func FuzzAggregatedBloomFilterUnmarshal(f *testing.F) {
	filter := core.NewAggregatedFilter(0)
	insertKey(f, &filter, []byte{0x01}, 0)
	valid := mustMarshal(f, &filter)

	f.Add([]byte(nil))
	f.Add([]byte{0x00})
	f.Add(valid)
	f.Add(valid[:25])
	// Header claiming a huge row count must not drive an unbounded allocation.
	hugeCount := make([]byte, 20)
	binary.BigEndian.PutUint32(hugeCount[16:], 0xFFFFFFFF)
	f.Add(hugeCount)

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded core.AggregatedBloomFilter
		_ = decoded.UnmarshalBinary(data) // must not panic

		var view core.AggregatedBloomFilterView
		if view.UnmarshalBinary(data) == nil {
			out := bitset.New(uint(core.NumBlocksPerFilter))
			_ = view.BlocksForKeysInto([][]byte{{0x01}}, out) // must not panic
		}
	})
}

func TestAggregatedBloomFilter_UnmarshalBinary_Allocs(t *testing.T) {
	// A fully populated filter is the worst case: all EventsBloomLength rows present.
	filter := core.NewAggregatedFilter(0)
	insertKey(t, &filter, []byte{0x01}, 0)

	data := mustMarshal(t, &filter)

	var decoded core.AggregatedBloomFilter
	allocs := testing.AllocsPerRun(20, func() {
		if err := decoded.UnmarshalBinary(data); err != nil {
			t.Fatal(err)
		}
	})
	// One backing []uint64 + one []bitset.BitSet header. Allow slack for
	// interface boxing in AllocsPerRun itself; current code is ~2.
	require.LessOrEqual(t, allocs, float64(8),
		"decode should allocate O(1) buffers, got %.0f", allocs)
}

func TestAggregatedBloomFilterView_MatchesDecodedFilter(t *testing.T) {
	const fromBlock = 3 * core.NumBlocksPerFilter
	filter := core.NewAggregatedFilter(fromBlock)
	keyA := []byte("key-a")
	keyB := []byte("key-b")
	insertKey(t, &filter, keyA, fromBlock)
	insertKey(t, &filter, keyA, fromBlock+17)
	insertKey(t, &filter, keyB, fromBlock+core.MaxBlockOffsetPerFilter)

	var view core.AggregatedBloomFilterView
	require.NoError(t, view.UnmarshalBinary(mustMarshal(t, &filter)))
	require.Equal(t, filter.FromBlock(), view.FromBlock())
	require.Equal(t, filter.ToBlock(), view.ToBlock())

	keySets := [][][]byte{
		nil,
		{keyA},
		{keyB},
		{keyA, keyB},
		{[]byte("absent")},
	}
	for _, keys := range keySets {
		want := blocksForKeys(t, &filter, keys)
		got := blocksForKeys(t, &view, keys)
		require.True(t, want.Equal(got), "view mismatch for keys %q", keys)
	}
}

// The stored value round-trip covers the accessors' assumption that the CBOR
// encoding of an AggregatedBloomFilter is a byte string of MarshalBinary output.
func TestGetAggregatedBloomFilter_RoundTrip(t *testing.T) {
	memDB := memory.New()
	const fromBlock = 5 * core.NumBlocksPerFilter
	filter := core.NewAggregatedFilter(fromBlock)
	key := []byte("round-trip")
	insertKey(t, &filter, key, fromBlock+42)
	require.NoError(t, core.WriteAggregatedBloomFilter(memDB, &filter))

	getters := []struct {
		name string
		get  func(r db.KeyValueReader, fromBlock, toBlock uint64) (queryFilter, error)
	}{
		{"filter", func(r db.KeyValueReader, fromBlock, toBlock uint64) (queryFilter, error) {
			decoded, err := core.GetAggregatedBloomFilter(r, fromBlock, toBlock)
			return &decoded, err
		}},
		{"view", func(r db.KeyValueReader, fromBlock, toBlock uint64) (queryFilter, error) {
			view, err := core.GetAggregatedBloomFilterView(r, fromBlock, toBlock)
			return &view, err
		}},
	}

	for _, getter := range getters {
		t.Run(getter.name, func(t *testing.T) {
			decoded, err := getter.get(memDB, filter.FromBlock(), filter.ToBlock())
			require.NoError(t, err)
			require.Equal(t, filter.FromBlock(), decoded.FromBlock())
			require.Equal(t, filter.ToBlock(), decoded.ToBlock())

			want := blocksForKeys(t, &filter, [][]byte{key})
			got := blocksForKeys(t, decoded, [][]byte{key})
			require.True(t, want.Equal(got))
			require.True(t, got.Test(42))
		})
	}
}
