package fastcbor_test

import (
	"runtime"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// weird is four words that encode themselves as one integer, the sum of the words: not a felt.
type weird [4]uint64

func (w weird) MarshalCBOR() ([]byte, error) { return fxamacker.Marshal(w[0] + w[1] + w[2] + w[3]) }

func (w *weird) UnmarshalCBOR(data []byte) error {
	var sum uint64
	if err := fxamacker.Unmarshal(data, &sum); err != nil {
		return err
	}
	*w = weird{sum}
	return nil
}

// alias has the encoding of a felt, as core.TransactionVersion does.
type alias felt.Felt

func (a alias) MarshalCBOR() ([]byte, error) { return fxamacker.Marshal(felt.Felt(a)) }

func (a *alias) UnmarshalCBOR(data []byte) error {
	return fxamacker.Unmarshal(data, (*felt.Felt)(a))
}

func TestTypesThatEncodeThemselves(t *testing.T) {
	type holder struct {
		W *weird
		V weird
		A *alias
		B alias
		F *felt.Felt
	}
	f := felt.NewFromUint64[felt.Felt](77)
	readByFast[holder](t, marshal(t, holder{
		W: &weird{1, 2, 3, 4}, V: weird{9}, A: (*alias)(f), F: f,
		B: alias(*felt.NewFromUint64[felt.Felt](5)),
	}))
}

type small struct {
	N uint64
	S string
}

func TestPointersToTheSameStruct(t *testing.T) {
	type siblings struct {
		A *small              `cbor:"a,omitempty"`
		B *small              `cbor:"b,omitempty"`
		C *small              `cbor:"c,omitempty"`
		D *struct{ X uint64 } `cbor:"d,omitempty"`
	}
	one := func(n uint64, s string) *small { return &small{N: n, S: s} }
	for name, in := range map[string]siblings{
		"all":     {A: one(1, "a"), B: one(2, "b"), C: one(3, "c")},
		"a only":  {A: one(1, "a")},
		"b only":  {B: one(2, "b")},
		"c only":  {C: one(3, "c")},
		"a and c": {A: one(1, "a"), C: one(3, "c")},
		"b and c": {B: one(2, "b"), C: one(3, "c")},
		"none":    {},
		"with d":  {A: one(1, "a"), B: one(2, "b"), D: &struct{ X uint64 }{X: 9}},
	} {
		t.Run(name, func(t *testing.T) { readByFast[siblings](t, marshal(t, in)) })
	}
}

type (
	childX struct{ A, B uint64 }
	childY struct {
		S string
		N uint64
	}
	parent struct {
		Steps uint64   `cbor:"steps"`
		X     *childX  `cbor:"x,omitempty"`
		Y     *childY  `cbor:"y,omitempty"`
		Names []string `cbor:"names,omitempty"`
		Self  *parent  `cbor:"self,omitempty"`
	}
)

func TestSmallStructsAllocatedWithTheirParent(t *testing.T) {
	type holder struct {
		P    *parent   `cbor:"p"`
		List []*parent `cbor:"list,omitempty"`
	}
	x, y := &childX{A: 1, B: 2}, &childY{S: "why", N: 3}
	for name, in := range map[string]holder{
		"both":   {P: &parent{Steps: 7, X: x, Y: y, Names: []string{"a", "b"}}},
		"x only": {P: &parent{Steps: 7, X: x}},
		"y only": {P: &parent{Steps: 7, Y: y}},
		"none":   {P: &parent{Steps: 7}},
		"nil":    {},
		"nested": {P: &parent{Steps: 1, X: x, Self: &parent{Steps: 2, Y: y}}},
		"in slice": {
			P:    &parent{Steps: 1},
			List: []*parent{{Steps: 2, X: x}, {Steps: 3, X: x, Y: y}, nil},
		},
	} {
		t.Run(name, func(t *testing.T) { readByFast[holder](t, marshal(t, in)) })
	}

	t.Run("a child outlives its parent", func(t *testing.T) {
		var got holder
		require.NoError(t, fastcbor.Unmarshal(marshal(t, holder{P: &parent{Steps: 1, X: x}}), &got))
		child := got.P.X
		got = holder{}
		for range 3 {
			runtime.GC()
		}
		require.Equal(t, *x, *child)
	})
}

func TestMapsKeyedByIntegers(t *testing.T) {
	type maps struct {
		MF  map[felt.Felt]uint64 `cbor:"mf"`
		MS  map[uint8]string     `cbor:"ms"`
		M32 map[uint32]uint64    `cbor:"m32"`
		M64 map[uint64]*small    `cbor:"m64"`
	}
	// a map of the given bytes under the given key
	under := func(name string, m []byte) []byte {
		return append(append([]byte{0xa1, 0x60 | byte(len(name))}, name...), m...)
	}
	supported := map[string][]byte{
		"normal": marshal(t, maps{
			M32: map[uint32]uint64{1: 10, 2: 20, 3: 30},
			M64: map[uint64]*small{1: {N: 1, S: "a"}, 7: {N: 7}},
		}),
		"large keys": marshal(t, maps{
			M32: map[uint32]uint64{64: 1, 1000: 2, 1<<32 - 1: 3},
			M64: map[uint64]*small{64: {N: 1}, 1 << 40: {N: 2, S: "x"}},
		}),
		"felt and small keys": marshal(t, maps{
			MF: map[felt.Felt]uint64{
				*felt.NewFromUint64[felt.Felt](5):       1,
				*felt.NewFromUint64[felt.Felt](1 << 40): 2,
			},
			MS: map[uint8]string{1: "a", 200: "b"},
		}),
		"empty":           marshal(t, maps{M32: map[uint32]uint64{}, M64: map[uint64]*small{}}),
		"nil":             marshal(t, maps{}),
		"duplicate key":   under("m32", []byte{0xa2, 0x01, 0x05, 0x01, 0x06}),
		"duplicate large": under("m32", []byte{0xa2, 0x18, 0x40, 0x05, 0x18, 0x40, 0x06}),
		"duplicate struct": under("m64", []byte{
			0xa2, 0x02, 0xa1, 0x61, 'n', 0x05, 0x02, 0xa1, 0x61, 's', 0x61, 'z',
		}),
		"null value": under("m64", []byte{0xa1, 0x01, 0xf6}),
	}
	for name, data := range supported {
		t.Run(name, func(t *testing.T) { readByFast[maps](t, data) })
	}

	// What fxamacker rejects it rejects, and what the fast reader leaves to it, it reads.
	for name, data := range map[string][]byte{
		"overflow 32 bits": under("m32", []byte{0xa1, 0x1b, 0, 0, 0, 1, 0, 0, 0, 0, 0x01}),
		"negative key":     under("m32", []byte{0xa1, 0x20, 0x01}),
		"text key":         under("m32", []byte{0xa1, 0x61, 'a', 0x01}),
	} {
		t.Run(name, func(t *testing.T) {
			var want, got maps
			require.Error(t, fxamacker.Unmarshal(data, &want))
			require.Error(t, fastcbor.Unmarshal(data, &got))
		})
	}
	t.Run("null key", func(t *testing.T) {
		// fxamacker keeps the previous key for a null one, which is not worth copying: it decides.
		require.True(t, compare[maps](t, under("m32", []byte{0xa1, 0xf6, 0x01})))
	})
}

func TestMapsOfBigElements(t *testing.T) {
	// A map keeps a pointer to an element above 128 bytes where it keeps the element itself below.
	type big struct {
		M32  map[uint32][200]byte `cbor:"m32"`
		M64  map[uint64][200]byte `cbor:"m64"`
		At   map[uint32][128]byte `cbor:"at"`
		Over map[uint64][129]byte `cbor:"over"`
	}
	fill := func(seed byte) (e [200]byte) {
		for i := range e {
			e[i] = seed + byte(i)
		}
		return e
	}
	in := big{
		M32:  map[uint32][200]byte{1: fill(1), 2: fill(2), 70: fill(70)},
		M64:  map[uint64][200]byte{1: fill(11), 1 << 33: fill(12)},
		At:   map[uint32][128]byte{1: {1}, 9: {9}, 100: {100}},
		Over: map[uint64][129]byte{1: {1}, 1 << 40: {2}},
	}
	var got big
	require.NoError(t, fastcbor.Unmarshal(marshal(t, in), &got))
	for range 3 {
		runtime.GC()
	}
	require.Equal(t, in, got)
	// The maps must still be usable.
	got.M32[999] = fill(99)
	require.Equal(t, fill(99), got.M32[999])
	require.Equal(t, fill(1), got.M32[1])
}

func TestStructsKeyedByIntegers(t *testing.T) {
	enc := func(v any) []byte { return marshal(t, v) }
	for name, data := range map[string][]byte{
		"both": enc(core.BlockTransactionsIndexes{
			Transactions: []int{0, 10, 250, 70000}, Receipts: []int{0, 5, 9},
		}),
		"transactions":  enc(core.BlockTransactionsIndexes{Transactions: []int{0, 1}}),
		"receipts":      enc(core.BlockTransactionsIndexes{Receipts: []int{4}}),
		"empty":         enc(core.BlockTransactionsIndexes{}),
		"null":          {0xf6},
		"unknown key":   {0xa3, 0x01, 0x81, 0x01, 0x07, 0x81, 0x02, 0x02, 0x81, 0x03},
		"text key":      {0xa2, 0x61, 'x', 0x01, 0x01, 0x81, 0x05},
		"duplicate key": {0xa2, 0x01, 0x81, 0x01, 0x01, 0x81, 0x02},
		"negative item": {0xa1, 0x01, 0x82, 0x01, 0x20},
		"null item":     {0xa1, 0x01, 0x82, 0xf6, 0x03},
	} {
		t.Run(name, func(t *testing.T) { compare[core.BlockTransactionsIndexes](t, data) })
	}
	t.Run("a block's indexes are read by the fast reader", func(t *testing.T) {
		readByFast[core.BlockTransactionsIndexes](t, enc(core.BlockTransactionsIndexes{
			Transactions: []int{0, 3}, Receipts: []int{1},
		}))
	})
}
