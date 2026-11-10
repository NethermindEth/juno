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

type plannerStateUpdate StateUpdate

func checkStatePlanner(t *testing.T, data []byte) {
	t.Helper()
	checkProjectionPlanner(t, data, func(v plannerStateUpdate) StateUpdate { return StateUpdate(v) })
	var direct, routed StateUpdate
	rest, err := DecodeStateUpdateFirst(data, &direct)
	wantRest, wantErr := fastcbor.UnmarshalFirst(data, &routed)
	require.Equal(t, wantErr, err)
	require.Equal(t, wantRest, rest)
	require.Equal(t, routed, direct)
}

func randomStateUpdate(rng *rand.Rand) StateUpdate {
	f := func() *felt.Felt {
		if rng.IntN(3) == 0 {
			return nil
		}
		return felt.NewFromUint64[felt.Felt](rng.Uint64())
	}
	fmap := func() map[felt.Felt]*felt.Felt {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make(map[felt.Felt]*felt.Felt)
		for n := rng.IntN(12); n > 0; n-- {
			out[felt.FromUint64[felt.Felt](rng.Uint64())] = f()
		}
		return out
	}
	value := StateUpdate{BlockHash: f(), NewRoot: f(), OldRoot: f()}
	if rng.IntN(4) == 0 {
		return value
	}
	d := &StateDiff{Nonces: fmap(), DeployedContracts: fmap(), DeclaredV1Classes: fmap(), ReplacedClasses: fmap()}
	if rng.IntN(3) != 0 {
		d.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt)
		for n := rng.IntN(8); n > 0; n-- {
			d.StorageDiffs[felt.FromUint64[felt.Felt](rng.Uint64())] = fmap()
		}
	}
	if rng.IntN(3) != 0 {
		d.DeclaredV0Classes = make([]*felt.Felt, rng.IntN(12))
		for i := range d.DeclaredV0Classes {
			d.DeclaredV0Classes[i] = f()
		}
	}
	if rng.IntN(3) != 0 {
		d.MigratedClasses = make(map[felt.SierraClassHash]felt.CasmClassHash)
		for n := rng.IntN(8); n > 0; n-- {
			d.MigratedClasses[felt.FromUint64[felt.SierraClassHash](rng.Uint64())] = felt.FromUint64[felt.CasmClassHash](rng.Uint64())
		}
	}
	value.StateDiff = d
	return value
}

func TestTypedStateUpdates(t *testing.T) {
	rng := rand.New(rand.NewPCG(713, 194))
	for range 128 {
		value := randomStateUpdate(rng)
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		checkStatePlanner(t, data)
		checkReceiptOwned(t, data, value)
		var direct StateUpdate
		require.NoError(t, DecodeStateUpdate(data, &direct))
		require.Equal(t, value, direct)
	}
	for _, data := range [][]byte{{0xa0}, {0xf6}, {0xf7}} {
		checkStatePlanner(t, data)
		checkReceiptOwned(t, data, StateUpdate{})
	}
	for _, item := range [][]byte{{0xf6}, {0xf7}, {0xa0}} {
		data := projectionMap(t, projectionPair{"StateDiff", item})
		var want StateUpdate
		if item[0] == 0xa0 {
			want.StateDiff = &StateDiff{}
		}
		checkStatePlanner(t, data)
		checkReceiptOwned(t, data, want)
	}
	// Every map/slice explicitly empty must remain non-nil after decoding.
	value := StateUpdate{StateDiff: &StateDiff{StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{}, Nonces: map[felt.Felt]*felt.Felt{}, DeployedContracts: map[felt.Felt]*felt.Felt{}, DeclaredV0Classes: []*felt.Felt{}, DeclaredV1Classes: map[felt.Felt]*felt.Felt{}, ReplacedClasses: map[felt.Felt]*felt.Felt{}, MigratedClasses: map[felt.SierraClassHash]felt.CasmClassHash{}}}
	data, err := cbor.Marshal(value)
	require.NoError(t, err)
	checkStatePlanner(t, data)
	checkReceiptOwned(t, data, value)
}

func TestTypedStateUpdateFailures(t *testing.T) {
	value := randomStateUpdate(rand.New(rand.NewPCG(63, 81)))
	// Ensure truncation tests cross allocated nested maps and a later field.
	value.StateDiff = &StateDiff{
		StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{{1}: {{2}: new(felt.Felt{3})}},
		Nonces:            map[felt.Felt]*felt.Felt{{4}: new(felt.Felt{5})},
		DeclaredV0Classes: []*felt.Felt{nil, new(felt.Felt{6})},
		MigratedClasses:   map[felt.SierraClassHash]felt.CasmClassHash{{7}: {8}},
	}
	data, err := cbor.Marshal(value)
	require.NoError(t, err)
	for end := range len(data) {
		checkStatePlanner(t, data[:end])
		var got StateUpdate
		rest, err := DecodeStateUpdateFirst(data[:end], &got)
		require.Error(t, err)
		require.Nil(t, rest)
		require.Equal(t, StateUpdate{}, got)
	}
	for _, value := range []StateUpdate{{BlockHash: new(felt.Felt)}, {OldRoot: new(felt.Felt)}, {NewRoot: new(felt.Felt)}, {StateDiff: &StateDiff{}}} {
		got := value
		rest, err := DecodeStateUpdateFirst([]byte{0xa0}, &got)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, value, got)
	}
	for _, decode := range []func([]byte, *StateUpdate) error{DecodeStateUpdate, fastcbor.Unmarshal[StateUpdate]} {
		var got StateUpdate
		require.ErrorIs(t, decode(append(bytes.Clone(data), 7), &got), fastcbor.ErrExtraneousData)
		require.Equal(t, StateUpdate{}, got)
	}
}

func stateDiffWire(t *testing.T, key string, value []byte) []byte {
	return projectionMap(t, projectionPair{"StateDiff", projectionMap(t, projectionPair{key, value})})
}

func TestTypedStateMapDuplicates(t *testing.T) {
	// These differently encoded keys decode to the same zero felt.
	short := []byte{0x80}
	wide := []byte{0x98, 4, 0x18, 0, 0x19, 0, 0, 0x1a, 0, 0, 0, 0, 0x1b, 0, 0, 0, 0, 0, 0, 0, 0}
	for _, field := range []string{"Nonces", "DeployedContracts", "DeclaredV1Classes", "ReplacedClasses"} {
		for _, last := range [][]byte{{0xf6}, {0xf7}, {0x80}, {0x84, 5, 6, 7, 8}} {
			body := append([]byte{0xa2}, short...)
			body = append(body, 0x84, 1, 2, 3, 4)
			body = append(body, wide...)
			body = append(body, last...)
			wire := stateDiffWire(t, field, body)
			checkStatePlanner(t, wire)
			var got StateUpdate
			require.NoError(t, DecodeStateUpdate(wire, &got))
			maps := map[string]map[felt.Felt]*felt.Felt{"Nonces": got.StateDiff.Nonces, "DeployedContracts": got.StateDiff.DeployedContracts, "DeclaredV1Classes": got.StateDiff.DeclaredV1Classes, "ReplacedClasses": got.StateDiff.ReplacedClasses}
			var want *felt.Felt
			if last[0] == 0x80 {
				want = new(felt.Felt)
			} else if last[0] == 0x84 {
				want = new(felt.Felt{5, 6, 7, 8})
			}
			require.Equal(t, map[felt.Felt]*felt.Felt{{}: want}, maps[field])
		}
	}
	// An outer duplicate replaces its inner map, including nil and empty maps.
	for _, last := range [][]byte{{0xf6}, {0xf7}, {0xa0}, {0xa1, 0x81, 2, 0x81, 9}} {
		body := append([]byte{0xa2}, short...)
		body = append(body, 0xa1, 0x81, 1, 0x81, 7)
		body = append(body, wide...)
		body = append(body, last...)
		wire := stateDiffWire(t, "StorageDiffs", body)
		checkStatePlanner(t, wire)
		var got StateUpdate
		require.NoError(t, DecodeStateUpdate(wire, &got))
		var want map[felt.Felt]*felt.Felt
		if last[0] == 0xa0 {
			want = map[felt.Felt]*felt.Felt{}
		} else if last[0] == 0xa1 {
			want = map[felt.Felt]*felt.Felt{{2}: new(felt.Felt{9})}
		}
		require.Equal(t, map[felt.Felt]map[felt.Felt]*felt.Felt{{}: want}, got.StateDiff.StorageDiffs)
	}
	body := append([]byte{0xa2}, short...)
	body = append(body, 0x81, 1)
	body = append(body, wide...)
	body = append(body, 0x84, 5, 6, 7, 8)
	wire := stateDiffWire(t, "MigratedClasses", body)
	checkStatePlanner(t, wire)
	var got StateUpdate
	require.NoError(t, DecodeStateUpdate(wire, &got))
	require.Equal(t, map[felt.SierraClassHash]felt.CasmClassHash{{}: {5, 6, 7, 8}}, got.StateDiff.MigratedClasses)
}

func TestTypedStateMapStorageOwnership(t *testing.T) {
	value := StateUpdate{StateDiff: &StateDiff{StorageDiffs: make(map[felt.Felt]map[felt.Felt]*felt.Felt)}}
	// Varying inner-map sizes force the shared value buffer to grow repeatedly.
	for i, count := range []int{1, 20, 1, 0, 5, 30} {
		inner := make(map[felt.Felt]*felt.Felt)
		for j := range count {
			inner[felt.Felt{uint64(j)}] = new(felt.Felt{7})
		}
		value.StateDiff.StorageDiffs[felt.Felt{uint64(i)}] = inner
	}
	data, err := cbor.Marshal(value)
	require.NoError(t, err)
	checkStatePlanner(t, data)
	checkReceiptOwned(t, data, value)
	var got StateUpdate
	require.NoError(t, DecodeStateUpdate(data, &got))
	first := got.StateDiff.StorageDiffs[felt.Felt{0}][felt.Felt{0}]
	*first = felt.Felt{99}
	for address, inner := range got.StateDiff.StorageDiffs {
		for key, value := range inner {
			if address == (felt.Felt{0}) && key == (felt.Felt{0}) {
				continue
			}
			require.Equal(t, felt.Felt{7}, *value, "distinct decoded values must not alias")
		}
	}
	// Keys differing in each individual limb remain separate, including zero.
	value = StateUpdate{StateDiff: &StateDiff{Nonces: map[felt.Felt]*felt.Felt{{}: nil, {1}: new(felt.Felt{1}), {0, 1}: new(felt.Felt{2}), {0, 0, 1}: new(felt.Felt{3}), {0, 0, 0, 1}: new(felt.Felt{4})}}}
	data, err = cbor.Marshal(value)
	require.NoError(t, err)
	checkStatePlanner(t, data)
	checkReceiptOwned(t, data, value)
}

func TestTypedStateUpdateWireCorners(t *testing.T) {
	for _, field := range []string{"StorageDiffs", "Nonces", "DeployedContracts", "DeclaredV0Classes", "DeclaredV1Classes", "ReplacedClasses", "MigratedClasses"} {
		for _, body := range [][]byte{{0xf6}, {0xf7}, {0xa0}, {0x80}, {0x9f, 0xff}, {0xbf, 0xff}, {0xa1, 0xf6, 0xf6}, {0xa1, 0x80, 0xf6}, {0xa1, 0x80, 0xf7}, {0xa1, 0x80, 0x80}, {0xa1, 0x80, 0xff}, {0xa1, 0x80, 0xa0}, {0xa1, 0x81, 0xff, 0x80}, {0xa1, 0x81, 1, 0x81, 2}, {0xb8, 1, 0x80, 0x80}} {
			checkStatePlanner(t, stateDiffWire(t, field, body))
		}
	}
	for _, field := range []string{"BlockHash", "NewRoot", "OldRoot", "StateDiff"} {
		data := projectionMap(t, projectionPair{field, []byte{0xf6}}, projectionPair{field, []byte{0xf6}})
		checkStatePlanner(t, data)
		var got StateUpdate
		require.ErrorIs(t, DecodeStateUpdate(data, &got), cursor.ErrUnsupported)
	}
	body := projectionMap(t, projectionPair{"nOnCeS", []byte{0xa1, 0x81, 1, 0x81, 7}}, projectionPair{"NONCES", []byte{0xf6}})
	data := projectionMap(t, projectionPair{"sTaTeDiFf", body})
	checkStatePlanner(t, data)
	var got StateUpdate
	require.ErrorIs(t, DecodeStateUpdate(data, &got), cursor.ErrUnsupported)
	require.Equal(t, StateUpdate{}, got)
	for _, depth := range []int{29, 30, 31, 32} {
		body := projectionMap(t, projectionPair{"StorageDiffs", []byte{0xa1, 0x80, 0xa1, 0x80, 0x80}}, projectionPair{"unknown", append(bytes.Repeat([]byte{0x81}, depth), 0)})
		data := projectionMap(t, projectionPair{"StateDiff", body})
		checkStatePlanner(t, data)
		var got StateUpdate
		err := DecodeStateUpdate(data, &got)
		if depth <= 30 {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, cursor.ErrDepth)
			require.Equal(t, StateUpdate{}, got)
		}
	}
	for _, data := range [][]byte{{0xd8, 100, 0xa0}, {0xd8}, {0xff}, {0x9f}, {0xbf}} {
		checkStatePlanner(t, data)
	}
}

func FuzzTypedStateUpdates(f *testing.F) {
	for _, seed := range []uint64{19, 54, 101} {
		value := randomStateUpdate(rand.New(rand.NewPCG(seed, 373)))
		data, err := cbor.Marshal(value)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	for _, seed := range [][]byte{{0xa0}, {0xf6}, {0xf7}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkStatePlanner(t, data) })
}
