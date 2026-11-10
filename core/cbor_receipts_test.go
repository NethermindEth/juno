package core

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
)

// Named shadows retain the planner route for comparisons through the public API.
type plannerReceipt TransactionReceipt
type plannerReceiptStatus receiptExecutionStatusProjection
type plannerReceiptEvents receiptEventsProjection

func checkReceiptPlanners(t *testing.T, data []byte) {
	t.Helper()
	checkProjectionPlanner(t, data, func(v plannerReceipt) TransactionReceipt { return TransactionReceipt(v) })
	checkProjectionPlanner(t, data, func(v *plannerReceipt) *TransactionReceipt { return (*TransactionReceipt)(v) })
	checkProjectionPlanner(t, data, func(v plannerReceiptStatus) receiptExecutionStatusProjection {
		return receiptExecutionStatusProjection(v)
	})
	checkProjectionPlanner(t, data, func(v plannerReceiptEvents) receiptEventsProjection { return receiptEventsProjection(v) })
	var direct, routed TransactionReceipt
	rest, err := DecodeTransactionReceiptFirst(data, &direct)
	wantRest, wantErr := fastcbor.UnmarshalFirst(data, &routed)
	require.Equal(t, wantErr, err)
	require.Equal(t, wantRest, rest)
	require.Equal(t, routed, direct)
	var directPtr, routedPtr *TransactionReceipt
	rest, err = DecodeTransactionReceiptPointerFirst(data, &directPtr)
	wantRest, wantErr = fastcbor.UnmarshalFirst(data, &routedPtr)
	require.Equal(t, wantErr, err)
	require.Equal(t, wantRest, rest)
	require.Equal(t, routedPtr, directPtr)
}

func checkReceiptOwned[T any](t *testing.T, data []byte, want T) {
	t.Helper()
	var oracle, got, zero T
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
	require.Equal(t, byte(8), input[len(data)])
	for i := range data {
		input[i] ^= 0xff
	}
	require.Equal(t, want, got, "receipt output must own its contents")
	var trailing T
	require.ErrorIs(t, fastcbor.Unmarshal(append(bytes.Clone(data), 7), &trailing), fastcbor.ErrExtraneousData)
	require.Equal(t, zero, trailing)
}

func randomReceipt(rng *rand.Rand) TransactionReceipt {
	f := func() *felt.Felt {
		if rng.IntN(3) == 0 {
			return nil
		}
		return felt.NewFromUint64[felt.Felt](rng.Uint64())
	}
	felts := func() felt.Slice[felt.Felt] {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make(felt.Slice[felt.Felt], rng.IntN(8))
		for i := range out {
			out[i] = felt.FromUint64[felt.Felt](rng.Uint64())
		}
		return out
	}
	address := func() (out eth.Address) {
		for i := range out {
			out[i] = byte(rng.Uint32())
		}
		return
	}
	r := TransactionReceipt{Fee: f(), FeeUnit: FeeUnit(rng.Uint32()), TransactionHash: f(), Reverted: rng.IntN(2) == 0, RevertReason: "reverted: αβ 🌍"}
	if rng.IntN(3) == 0 {
		r.RevertReason = ""
	}
	if rng.IntN(3) != 0 {
		r.Events = make([]*Event, rng.IntN(8))
		for i := range r.Events {
			if rng.IntN(3) != 0 {
				r.Events[i] = &Event{From: f(), Keys: felts(), Data: felts()}
			}
		}
	}
	if rng.IntN(3) != 0 {
		r.L2ToL1Message = make([]*L2ToL1Message, rng.IntN(8))
		for i := range r.L2ToL1Message {
			if rng.IntN(3) != 0 {
				r.L2ToL1Message[i] = &L2ToL1Message{From: f(), Payload: felts(), To: address()}
			}
		}
	}
	if rng.IntN(3) != 0 {
		r.L1ToL2Message = &L1ToL2Message{From: address(), Nonce: f(), Payload: felts(), Selector: f(), To: f()}
	}
	if rng.IntN(3) != 0 {
		r.ExecutionResources = &ExecutionResources{MemoryHoles: rng.Uint64(), Steps: rng.Uint64(), BuiltinInstanceCounter: BuiltinInstanceCounter{
			Pedersen: rng.Uint64(), RangeCheck: rng.Uint64(), Bitwise: rng.Uint64(), Output: rng.Uint64(), Ecsda: rng.Uint64(), EcOp: rng.Uint64(), Keccak: rng.Uint64(), Poseidon: rng.Uint64(), SegmentArena: rng.Uint64(), AddMod: rng.Uint64(), MulMod: rng.Uint64(), RangeCheck96: rng.Uint64(),
		}}
		if rng.IntN(3) != 0 {
			r.ExecutionResources.DataAvailability = &DataAvailability{L1Gas: rng.Uint64(), L1DataGas: rng.Uint64()}
		}
		if rng.IntN(3) != 0 {
			r.ExecutionResources.TotalGasConsumed = &GasConsumed{L1Gas: rng.Uint64(), L1DataGas: rng.Uint64(), L2Gas: rng.Uint64()}
		}
	}
	return r
}

func TestTypedReceipts(t *testing.T) {
	rng := rand.New(rand.NewPCG(131, 917))
	for range 128 {
		value := randomReceipt(rng)
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		checkReceiptPlanners(t, data)
		checkReceiptOwned(t, data, value)
		checkReceiptOwned(t, data, &value)
		checkReceiptOwned(t, data, receiptExecutionStatusProjection{Reverted: value.Reverted, RevertReason: value.RevertReason})
		checkReceiptOwned(t, data, receiptEventsProjection{Events: value.Events, TransactionHash: value.TransactionHash})
		var direct TransactionReceipt
		require.NoError(t, DecodeTransactionReceipt(data, &direct))
		require.Equal(t, value, direct)
		var directPtr *TransactionReceipt
		require.NoError(t, DecodeTransactionReceiptPointer(data, &directPtr))
		require.Equal(t, &value, directPtr)
	}
	for _, data := range [][]byte{{0xa0}, {0xf6}, {0xf7}} {
		checkReceiptPlanners(t, data)
		checkReceiptOwned(t, data, TransactionReceipt{})
		var want *TransactionReceipt
		if data[0] == 0xa0 {
			want = &TransactionReceipt{}
		}
		checkReceiptOwned(t, data, want)
	}
	// Empty maps and nulls exercise the fallback after the shared slice storage
	// fills; fully encoded records above exercise the dense allocation path.
	for _, key := range []string{"Events", "L2ToL1Message"} {
		for _, array := range [][]byte{{0x80}, {0xf6}, {0xf7}, {0x85, 0xa0, 0xf6, 0xa0, 0xf7, 0xa0}} {
			data := projectionMap(t, projectionPair{key, array})
			checkReceiptPlanners(t, data)
			var want TransactionReceipt
			require.NoError(t, cbor.Unmarshal(data, &want))
			checkReceiptOwned(t, data, want)
		}
	}
}

func TestTypedReceiptFailures(t *testing.T) {
	value := randomReceipt(rand.New(rand.NewPCG(521, 33)))
	data, err := cbor.Marshal(value)
	require.NoError(t, err)
	for end := range len(data) {
		input := data[:end]
		checkReceiptPlanners(t, input)
		var got TransactionReceipt
		rest, err := DecodeTransactionReceiptFirst(input, &got)
		require.Error(t, err)
		require.Nil(t, rest)
		require.Equal(t, TransactionReceipt{}, got)
		var ptr *TransactionReceipt
		require.Error(t, DecodeTransactionReceiptPointer(input, &ptr))
		require.Nil(t, ptr)
	}
	for _, value := range []TransactionReceipt{{Events: []*Event{}}, {L2ToL1Message: []*L2ToL1Message{}}, {ExecutionResources: &ExecutionResources{}}, {FeeUnit: 1}, {Reverted: true}, {RevertReason: "x"}, {TransactionHash: new(felt.Felt)}} {
		got := value
		rest, err := DecodeTransactionReceiptFirst([]byte{0xa0}, &got)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, value, got)
	}
	ptr := &TransactionReceipt{}
	original := ptr
	rest, err := DecodeTransactionReceiptPointerFirst([]byte{0xa0}, &ptr)
	require.ErrorIs(t, err, fastcbor.ErrUnsupported)
	require.Nil(t, rest)
	require.Same(t, original, ptr)
	for _, decode := range []func([]byte, *TransactionReceipt) error{DecodeTransactionReceipt, fastcbor.Unmarshal[TransactionReceipt]} {
		var got TransactionReceipt
		require.ErrorIs(t, decode(append(bytes.Clone(data), 7), &got), fastcbor.ErrExtraneousData)
		require.Equal(t, TransactionReceipt{}, got)
	}
	var trailing *TransactionReceipt
	require.ErrorIs(t, DecodeTransactionReceiptPointer(append(bytes.Clone(data), 7), &trailing), fastcbor.ErrExtraneousData)
	require.Nil(t, trailing)
	// Errors after allocating outputs must clear those outputs too.
	for _, bad := range [][]byte{{0xff}, {0x19, 1, 0}, {0x61, 'x'}} {
		input := projectionMap(t, projectionPair{"Events", []byte{0x81, 0xa0}}, projectionPair{"FeeUnit", bad})
		checkReceiptPlanners(t, input)
		var got TransactionReceipt
		require.Error(t, DecodeTransactionReceipt(input, &got))
		require.Equal(t, TransactionReceipt{}, got)
	}
}

func TestTypedReceiptWireCorners(t *testing.T) {
	for _, value := range [][]byte{{0}, {0x18, 255}, {0x19, 0, 255}, {0x1b, 0, 0, 0, 0, 0, 0, 0, 255}, {0x19, 1, 0}, {0x20}, {0xf6}, {0xf7}} {
		checkReceiptPlanners(t, projectionMap(t, projectionPair{"fEeUnIt", value}))
	}
	for _, key := range []string{"Reverted", "RevertReason", "Events", "ExecutionResources", "L1ToL2Message", "L2ToL1Message", "Fee", "TransactionHash"} {
		for _, value := range [][]byte{{0xf6}, {0xf7}, {0xa0}, {0x80}, {0xf5}, {0xf4}, {0x61, 0xff}, {0x9f, 0xff}, {0xd8, 100, 0xa0}, {0x18}} {
			checkReceiptPlanners(t, projectionMap(t, projectionPair{key, value}))
		}
	}
	for _, key := range []string{"From", "To"} {
		for _, size := range []int{0, 19, 20, 21} {
			body, err := cbor.Marshal(make([]byte, size))
			require.NoError(t, err)
			msg := projectionMap(t, projectionPair{key, body})
			outer := "L1ToL2Message"
			if key == "To" {
				outer = "L2ToL1Message"
				msg = append([]byte{0x81}, msg...)
			}
			checkReceiptPlanners(t, projectionMap(t, projectionPair{outer, msg}))
		}
	}
	for _, first := range [][]byte{{0xf4}, {0xf6}, {0xf7}, {0xf5}} {
		data := projectionMap(t, projectionPair{"Reverted", first}, projectionPair{"REVERTED", []byte{0x61, 'x'}})
		checkReceiptPlanners(t, data)
		var got receiptExecutionStatusProjection
		require.NoError(t, fastcbor.Unmarshal(data, &got))
		require.Equal(t, first[0] == 0xf5, got.Reverted)
		var full TransactionReceipt
		require.ErrorIs(t, DecodeTransactionReceipt(data, &full), cursor.ErrUnsupported)
	}
	// Depth is global across the record and is restored after known containers.
	for _, depth := range []int{30, 31, 32, 33} {
		nested := append(bytes.Repeat([]byte{0x81}, depth), 0)
		input := projectionMap(t, projectionPair{"Events", []byte{0x81, 0xa0}}, projectionPair{"unknown", nested})
		checkReceiptPlanners(t, input)
		var got TransactionReceipt
		err := DecodeTransactionReceipt(input, &got)
		if depth <= 31 {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, cursor.ErrDepth)
			require.Equal(t, TransactionReceipt{}, got)
		}
	}
}

func TestTypedReceiptProjectionTags(t *testing.T) {
	data := projectionMap(t, projectionPair{"Reverted", []byte{0xf5}}, projectionPair{"RevertReason", []byte{0x62, 'o', 'k'}})
	for _, head := range [][]byte{{0xc0}, {0xc1}, {0xc2}, {0xc3}, {0xc4}, {0xd8, 100}, {0xd9, 0xff, 0xff}, {0xda, 0, 1, 0, 0}, {0xdb, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, {0xd8}, {0xdf}} {
		checkReceiptPlanners(t, head)
		checkReceiptPlanners(t, append(bytes.Clone(head), data...))
		checkReceiptPlanners(t, append(append(bytes.Clone(head), 0xd8, 100), data...))
	}
	checkReceiptOwned(t, append([]byte{0xd8, 100}, data...), receiptExecutionStatusProjection{Reverted: true, RevertReason: "ok"})
	for _, value := range []receiptEventsProjection{{Events: []*Event{}}, {TransactionHash: new(felt.Felt)}} {
		got := value
		rest, err := fastcbor.UnmarshalFirst([]byte{0xa0}, &got)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, value, got)
	}
}

func FuzzTypedReceipts(f *testing.F) {
	value := randomReceipt(rand.New(rand.NewPCG(418, 371)))
	data, err := cbor.Marshal(value)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{data, {0xa0}, {0xf6}, {0xf7}, {0xa1, 0x66, 'E', 'v', 'e', 'n', 't', 's', 0x81, 0xa0}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkReceiptPlanners(t, data) })
}
