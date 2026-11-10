package core_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/stretchr/testify/require"
)

// A different named type exercises the original planner through its public API.
type plannerIndexes core.BlockTransactionsIndexes

func checkIndexPlanner(t *testing.T, data []byte) {
	t.Helper()
	var planned plannerIndexes
	var got, routed core.BlockTransactionsIndexes
	before := bytes.Clone(data)
	wantRest, wantErr := fastcbor.UnmarshalFirst(data, &planned)
	rest, err := core.DecodeBlockTransactionsIndexesFirst(data, &got)
	routedRest, routedErr := fastcbor.UnmarshalFirst(data, &routed)
	require.Equal(t, before, data)
	if wantErr == nil {
		require.NoError(t, err, "%x", data)
		require.NoError(t, routedErr, "%x", data)
	} else {
		require.EqualError(t, err, wantErr.Error(), "%x", data)
		require.EqualError(t, routedErr, wantErr.Error(), "%x", data)
	}
	require.Equal(t, wantRest, rest, "%x", data)
	require.Equal(t, rest, routedRest)
	require.Equal(t, core.BlockTransactionsIndexes(planned), got, "%x", data)
	require.Equal(t, got, routed)
}

func checkTypedIndexes(t *testing.T, data []byte, want core.BlockTransactionsIndexes) {
	t.Helper()
	var oracle core.BlockTransactionsIndexes
	require.NoError(t, cbor.Unmarshal(data, &oracle))
	require.Equal(t, want, oracle)
	checkIndexPlanner(t, data)
	for _, decode := range []func([]byte, *core.BlockTransactionsIndexes) error{
		core.DecodeBlockTransactionsIndexes, fastcbor.Unmarshal[core.BlockTransactionsIndexes],
	} {
		var got core.BlockTransactionsIndexes
		require.NoError(t, decode(data, &got))
		require.Equal(t, want, got)
		var trailing core.BlockTransactionsIndexes
		require.ErrorIs(t, decode(append(bytes.Clone(data), 7), &trailing), fastcbor.ErrExtraneousData)
		require.Equal(t, core.BlockTransactionsIndexes{}, trailing)
	}
	input := append(bytes.Clone(data), 7)
	before := bytes.Clone(input)
	var got core.BlockTransactionsIndexes
	rest, err := core.DecodeBlockTransactionsIndexesFirst(input, &got)
	require.NoError(t, err)
	require.Equal(t, before, input)
	require.Equal(t, []byte{7}, rest)
	rest[0] = 8
	require.Equal(t, byte(8), input[len(data)])
	for i := range data {
		input[i] ^= 0xff
	}
	require.Equal(t, want, got, "index storage must not borrow the encoded input")
}

func TestTypedBlockTransactionsIndexes(t *testing.T) {
	rng := rand.New(rand.NewPCG(931, 382))
	for range 128 {
		var value core.BlockTransactionsIndexes
		for _, dst := range []*[]int{&value.Transactions, &value.Receipts} {
			count := rng.IntN(65)
			if count == 0 {
				continue
			}
			*dst = make([]int, count)
			for i := range *dst {
				(*dst)[i] = int(rng.Uint64())
			}
		}
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		checkTypedIndexes(t, data, value)
	}
	for _, data := range [][]byte{{0xa0}, {0xf6}, {0xf7}, {0xa2, 1, 0xf6, 2, 0xf7}} {
		checkTypedIndexes(t, data, core.BlockTransactionsIndexes{})
	}
	checkTypedIndexes(t, []byte{0xa2, 1, 0x80, 2, 0x80}, core.BlockTransactionsIndexes{Transactions: []int{}, Receipts: []int{}})
	checkTypedIndexes(t, []byte{0xa1, 1, 0x84, 0xf6, 0xf7, 0x20, 0x18, 24}, core.BlockTransactionsIndexes{Transactions: []int{0, 0, -1, 24}})
	for _, value := range []core.BlockTransactionsIndexes{{Transactions: []int{}}, {Receipts: []int{}}, {Transactions: []int{0}}, {Receipts: []int{1}}} {
		got := value
		rest, err := core.DecodeBlockTransactionsIndexesFirst([]byte{0xa0}, &got)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, value, got)
	}
}

func indexHead(major byte, value uint64, width int) []byte {
	switch width {
	case 0:
		return []byte{major | byte(value)}
	case 1:
		return []byte{major | 24, byte(value)}
	case 2:
		return binary.BigEndian.AppendUint16([]byte{major | 25}, uint16(value))
	case 4:
		return binary.BigEndian.AppendUint32([]byte{major | 26}, uint32(value))
	default:
		return binary.BigEndian.AppendUint64([]byte{major | 27}, value)
	}
}

func TestTypedIndexesWidthsAndBoundaries(t *testing.T) {
	for _, mapWidth := range []int{0, 1, 2, 4, 8} {
		for _, keyWidth := range []int{0, 1, 2, 4, 8} {
			for _, arrayWidth := range []int{0, 1, 2, 4, 8} {
				for _, valueWidth := range []int{0, 1, 2, 4, 8} {
					data := indexHead(0xa0, 1, mapWidth)
					data = append(data, indexHead(0, 2, keyWidth)...)
					data = append(data, indexHead(0x80, 2, arrayWidth)...)
					data = append(data, indexHead(0, 23, valueWidth)...)
					data = append(data, indexHead(0x20, 23, valueWidth)...)
					checkTypedIndexes(t, data, core.BlockTransactionsIndexes{Receipts: []int{23, -24}})
				}
			}
		}
	}
	for _, major := range []byte{0, 0x20} {
		for _, magnitude := range []uint64{math.MaxInt64, uint64(math.MaxInt64) + 1, math.MaxUint64} {
			data := append([]byte{0xa1, 1, 0x81}, indexHead(major, magnitude, 8)...)
			checkIndexPlanner(t, data)
			var got core.BlockTransactionsIndexes
			err := core.DecodeBlockTransactionsIndexes(data, &got)
			if magnitude > math.MaxInt64 {
				require.ErrorIs(t, err, cursor.ErrType)
				require.Equal(t, core.BlockTransactionsIndexes{}, got)
			} else {
				require.NoError(t, err)
				want := int(magnitude)
				if major != 0 {
					want = ^want
				}
				require.Equal(t, []int{want}, got.Transactions)
			}
		}
	}
}

func TestTypedIndexesKeysAndFailures(t *testing.T) {
	// Text "1", negative integer and unknown unsigned keys are ignored. Keys 1
	// and 2 are deliberately reversed and a duplicate value is not an int array.
	data := []byte{0xa6, 0x61, '1', 0x80, 0x20, 0x80, 3, 0x82, 0x80, 0xf6, 2, 0x81, 7, 1, 0x81, 9, 1, 0xf5}
	var got core.BlockTransactionsIndexes
	require.NoError(t, core.DecodeBlockTransactionsIndexes(data, &got))
	require.Equal(t, core.BlockTransactionsIndexes{Transactions: []int{9}, Receipts: []int{7}}, got)
	checkIndexPlanner(t, data)
	for _, first := range []byte{0xf6, 0xf7, 0x80} {
		data := []byte{0xa2, 1, first, 1, 0x81, 9}
		checkIndexPlanner(t, data)
		var got core.BlockTransactionsIndexes
		require.NoError(t, core.DecodeBlockTransactionsIndexes(data, &got))
		if first == 0x80 {
			require.Equal(t, []int{}, got.Transactions)
		} else {
			require.Nil(t, got.Transactions)
		}
	}
	for end := range len(data) {
		var got core.BlockTransactionsIndexes
		rest, err := core.DecodeBlockTransactionsIndexesFirst(data[:end], &got)
		require.Error(t, err)
		require.Nil(t, rest)
		require.Equal(t, core.BlockTransactionsIndexes{}, got)
		checkIndexPlanner(t, data[:end])
	}
	for _, tail := range [][]byte{{2, 0x82, 1, 0x18}, {3, 0x18}, {0x61, 0xff, 0}, {0xf5, 0}, {0x3b, 0x80, 0, 0, 0, 0, 0, 0, 0, 0}, {0x1b, 0x80, 0, 0, 0, 0, 0, 0, 0, 0}} {
		data := append([]byte{0xa2, 1, 0x81, 7}, tail...)
		checkIndexPlanner(t, data)
		var got core.BlockTransactionsIndexes
		require.Error(t, core.DecodeBlockTransactionsIndexes(data, &got))
		require.Equal(t, core.BlockTransactionsIndexes{}, got)
	}
}

func FuzzTypedBlockTransactionsIndexes(f *testing.F) {
	for _, data := range [][]byte{{}, {0xa0}, {0xf6}, {0xa2, 1, 0x82, 1, 2, 2, 0x81, 3}, {0xa1, 1, 0x84, 0xf6, 0xf7, 0x20, 0x18, 24}} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkIndexPlanner(t, data) })
}

func TestTypedIndexesWholeRecordDepth(t *testing.T) {
	for _, depth := range []int{30, 31, 32, 33} {
		// A populated known array must leave its nesting level before the
		// unknown nested value. The enclosing record itself counts as one.
		data := append([]byte{0xa2, 1, 0x81, 7, 3}, bytes.Repeat([]byte{0x81}, depth)...)
		data = append(data, 0)
		checkIndexPlanner(t, data)
		var got core.BlockTransactionsIndexes
		err := core.DecodeBlockTransactionsIndexes(data, &got)
		if depth <= 31 {
			require.NoError(t, err)
			require.Equal(t, core.BlockTransactionsIndexes{Transactions: []int{7}}, got)
		} else {
			require.ErrorIs(t, err, cursor.ErrDepth)
			require.Equal(t, core.BlockTransactionsIndexes{}, got)
		}
	}
}
