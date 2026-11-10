package core

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
)

func checkTypedProjection[T comparable](t *testing.T, data []byte, want T) {
	t.Helper()
	var zero, got, oracle T
	require.NoError(t, cbor.Unmarshal(data, &oracle))
	require.Equal(t, want, oracle)
	input := append(bytes.Clone(data), 7)
	before := bytes.Clone(input)
	rest, err := fastcbor.UnmarshalFirst(input, &got)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, before, input)
	require.Equal(t, []byte{7}, rest)
	rest[0] = 8
	require.Equal(t, byte(8), input[len(data)], "suffix must refer to the original input")
	for i := range data {
		input[i] ^= 0xff
	}
	require.Equal(t, want, got, "selected fields must own their data")
	var trailing T
	require.ErrorIs(t, fastcbor.Unmarshal(append(bytes.Clone(data), 7), &trailing), fastcbor.ErrExtraneousData)
	require.Equal(t, zero, trailing)
	if want != zero {
		unchanged := got
		rest, err = fastcbor.UnmarshalFirst(data, &got)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, unchanged, got)
	}
}

func TestTypedHeaderProjections(t *testing.T) {
	rng := rand.New(rand.NewPCG(431, 712))
	for i := range 64 {
		header := Header{
			Hash:             felt.NewFromUint64[felt.Felt](rng.Uint64()),
			GlobalStateRoot:  felt.NewFromUint64[felt.Felt](rng.Uint64()),
			TransactionCount: rng.Uint64(), Timestamp: rng.Uint64(),
			ProtocolVersion: "0.13.α", Signatures: [][]*felt.Felt{nil, {}, {nil, felt.NewFromUint64[felt.Felt](3)}},
			L1DataGasPrice: &GasPrice{PriceInWei: felt.NewFromUint64[felt.Felt](5)},
		}
		if i%2 == 0 {
			header.Hash = nil
		}
		if i%3 == 0 {
			header.GlobalStateRoot = nil
		}
		if i%4 == 0 {
			header.Timestamp, header.TransactionCount = 0, 0
		}
		data, err := cbor.Marshal(header)
		require.NoError(t, err)
		checkTypedProjection(t, data, headerHashProjection{Hash: header.Hash})
		checkTypedProjection(t, data, headerGlobalStateRootProjection{GlobalStateRoot: header.GlobalStateRoot})
		checkTypedProjection(t, data, headerTransactionCountProjection{TransactionCount: header.TransactionCount})
		checkTypedProjection(t, data, headerTimestampProjection{Timestamp: &header.Timestamp})
		checkTypedProjection(t, data, headerHashAndStateRootProjection{Hash: header.Hash, GlobalStateRoot: header.GlobalStateRoot})
	}
}

type projectionPair struct {
	key   any
	value []byte
}

func projectionMap(t *testing.T, pairs ...projectionPair) []byte {
	t.Helper()
	if len(pairs) >= 24 {
		t.Fatal("test map head needs a wider encoding")
	}
	out := []byte{0xa0 | byte(len(pairs))}
	for _, pair := range pairs {
		key, err := cbor.Marshal(pair.key)
		require.NoError(t, err)
		out = append(out, key...)
		out = append(out, pair.value...)
	}
	return out
}

func TestTypedHeaderProjectionSelection(t *testing.T) {
	// Ignored Header fields can have a different type. Selection still consumes
	// their full values and rejects malformed tails after the selected field.
	data := projectionMap(t,
		projectionPair{"Hash", []byte{0x82, 1, 2}},
		projectionPair{"tImEsTaMp", []byte{0x1b, 0, 0, 0, 0, 0, 0, 0, 23}},
		projectionPair{uint64(999), []byte{0x82, 0x80, 0xf6}},
		projectionPair{"Signatures", []byte{0x61, 'x'}},
	)
	value := uint64(23)
	checkTypedProjection(t, data, headerTimestampProjection{Timestamp: &value})
	for _, first := range [][]byte{{0xf6}, {0xf7}, {0}} {
		data := projectionMap(t, projectionPair{"Timestamp", first}, projectionPair{"TIMESTAMP", []byte{9}})
		var got headerTimestampProjection
		require.NoError(t, fastcbor.Unmarshal(data, &got))
		if first[0] == 0 {
			require.Equal(t, uint64(0), *got.Timestamp)
		} else {
			require.Nil(t, got.Timestamp)
		}
	}
	for _, data := range [][]byte{{0xa0}, {0xf6}, {0xf7}} {
		checkTypedProjection(t, data, headerHashProjection{})
		checkTypedProjection(t, data, headerGlobalStateRootProjection{})
		checkTypedProjection(t, data, headerTransactionCountProjection{})
		checkTypedProjection(t, data, headerTimestampProjection{})
		checkTypedProjection(t, data, headerHashAndStateRootProjection{})
	}
}

func checkProjectionFailures[T comparable](t *testing.T, data []byte) {
	t.Helper()
	var zero T
	for end := 0; end < len(data); end++ {
		var got T
		rest, err := fastcbor.UnmarshalFirst(data[:end], &got)
		require.Error(t, err)
		require.Nil(t, rest)
		require.Equal(t, zero, got, "partial result at prefix %d", end)
	}
}

func TestTypedHeaderProjectionFailures(t *testing.T) {
	data, err := cbor.Marshal(Header{Hash: felt.NewFromUint64[felt.Felt](7), GlobalStateRoot: felt.NewFromUint64[felt.Felt](11), Timestamp: 32, TransactionCount: 100})
	require.NoError(t, err)
	checkProjectionFailures[headerHashProjection](t, data)
	checkProjectionFailures[headerGlobalStateRootProjection](t, data)
	checkProjectionFailures[headerTransactionCountProjection](t, data)
	checkProjectionFailures[headerTimestampProjection](t, data)
	checkProjectionFailures[headerHashAndStateRootProjection](t, data)

	for _, tail := range [][]byte{{0x18}, {0xff}, {0x9f}} {
		data := projectionMap(t, projectionPair{"Timestamp", []byte{1}}, projectionPair{"ignored", tail})
		var got headerTimestampProjection
		rest, err := fastcbor.UnmarshalFirst(data, &got)
		require.Error(t, err)
		require.Nil(t, rest)
		require.Nil(t, got.Timestamp)
	}
	var got headerTimestampProjection
	data = projectionMap(t, projectionPair{"Timestamp", []byte{0x61, 'x'}})
	require.ErrorIs(t, fastcbor.Unmarshal(data, &got), cursor.ErrType)
	require.Nil(t, got.Timestamp)
}

// Distinct named types retain the original planner route, allowing the public
// API to compare both implementations on exactly the same malformed inputs.
type plannerHashProjection headerHashProjection
type plannerRootProjection headerGlobalStateRootProjection
type plannerCountProjection headerTransactionCountProjection
type plannerTimestampProjection headerTimestampProjection
type plannerHashRootProjection headerHashAndStateRootProjection

func checkProjectionPlanner[T, P any](t *testing.T, data []byte, convert func(P) T) {
	t.Helper()
	var typed T
	var planned P
	before := bytes.Clone(data)
	typedRest, typedErr := fastcbor.UnmarshalFirst(data, &typed)
	plannedRest, plannedErr := fastcbor.UnmarshalFirst(data, &planned)
	require.Equal(t, before, data)
	if plannedErr == nil {
		require.NoError(t, typedErr, "%x", data)
	} else {
		require.EqualError(t, typedErr, plannedErr.Error(), "%x", data)
	}
	require.Equal(t, plannedRest, typedRest, "%x", data)
	require.Equal(t, convert(planned), typed, "%x", data)
}

func TestTypedHeaderProjectionPlannerParity(t *testing.T) {
	data, err := cbor.Marshal(sampleHeader())
	require.NoError(t, err)
	seeds := [][]byte{nil, {}, {0xf6}, {0xf7}, {0xa0}, data,
		projectionMap(t, projectionPair{"Timestamp", []byte{1}}, projectionPair{"TIMESTAMP", []byte{0x61, 'x'}}),
		projectionMap(t, projectionPair{"GlobalStateRoot", []byte{0x80}}, projectionPair{"hAsH", []byte{0x80}}, projectionPair{"Hash", []byte{0xf5}}),
	}
	for _, head := range [][]byte{{0xc0}, {0xc1}, {0xc2}, {0xc3}, {0xc4}, {0xd8, 100}, {0xd9, 0xff, 0xff}, {0xda, 0, 1, 0, 0}, {0xdb, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, {0xd8}, {0xdf}} {
		seeds = append(seeds, bytes.Clone(head), append(bytes.Clone(head), data...))
		seeds = append(seeds, append(append(bytes.Clone(head), 0xd8, 100), data...))
	}
	rng := rand.New(rand.NewPCG(215, 779))
	for range 256 {
		input := bytes.Clone(seeds[rng.IntN(len(seeds))])
		if len(input) > 0 {
			input[rng.IntN(len(input))] = byte(rng.Uint32())
		}
		seeds = append(seeds, input)
	}
	for _, input := range seeds {
		checkAllProjectionPlanners(t, input)
	}
}

func checkAllProjectionPlanners(t *testing.T, input []byte) {
	t.Helper()
	checkProjectionPlanner(t, input, func(v plannerHashProjection) headerHashProjection { return headerHashProjection(v) })
	checkProjectionPlanner(t, input, func(v plannerRootProjection) headerGlobalStateRootProjection {
		return headerGlobalStateRootProjection(v)
	})
	checkProjectionPlanner(t, input, func(v plannerCountProjection) headerTransactionCountProjection {
		return headerTransactionCountProjection(v)
	})
	checkProjectionPlanner(t, input, func(v plannerTimestampProjection) headerTimestampProjection { return headerTimestampProjection(v) })
	checkProjectionPlanner(t, input, func(v plannerHashRootProjection) headerHashAndStateRootProjection {
		return headerHashAndStateRootProjection(v)
	})
}

func FuzzTypedHeaderProjections(f *testing.F) {
	data, err := cbor.Marshal(sampleHeader())
	if err != nil {
		f.Fatal(err)
	}
	for _, input := range [][]byte{data, {}, {0xa0}, {0xf6}, {0xd8, 100, 0xa0}} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input []byte) { checkAllProjectionPlanners(t, input) })
}
