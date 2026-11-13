// These tests pin the plans of fastcbor to fxamacker. Each type below is written by fxamacker and
// read by fastcbor, and the two must give the same value, or fail alike.

package cbor_test

import (
	"math/big"
	"runtime"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// limbSum is four limbs that encode themselves as one integer, the sum of the limbs, so it is not a
// felt.
type limbSum [4]uint64

func (w limbSum) MarshalCBOR() ([]byte, error) {
	return fxamacker.Marshal(w[0] + w[1] + w[2] + w[3])
}

func (w *limbSum) UnmarshalCBOR(data []byte) error {
	var sum uint64
	if err := fxamacker.Unmarshal(data, &sum); err != nil {
		return err
	}
	*w = limbSum{sum}
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
		W *limbSum
		V limbSum
		A *alias
		B alias
		F *felt.Felt
	}
	f := felt.NewFromUint64[felt.Felt](77)
	readByFastcbor[holder](t, marshal(t, holder{
		W: &limbSum{1, 2, 3, 4}, V: limbSum{9}, A: (*alias)(f), F: f,
		B: alias(*felt.NewFromUint64[felt.Felt](5)),
	}))
}

type counter uint

func TestScalarsAndBigInts(t *testing.T) {
	type scalars struct {
		U8     uint8
		U16    uint16
		U32    uint32
		Named  counter
		Flag   bool
		Raw    []byte
		Hash   [4]byte
		Big    *big.Int
		BigVal big.Int
	}
	huge, _ := new(big.Int).SetString("123456789012345678901234567890123456789", 10)
	for name, in := range map[string]scalars{
		"zero":     {},
		"small":    {U8: 200, U16: 60000, U32: 4000000000, Named: 7, Flag: true},
		"bytes":    {Raw: []byte{1, 2, 3}, Hash: [4]byte{9, 8, 7, 6}},
		"empty":    {Raw: []byte{}},
		"big":      {Big: big.NewInt(5), BigVal: *big.NewInt(6)},
		"huge":     {Big: huge, BigVal: *huge},
		"negative": {Big: big.NewInt(-5), BigVal: *new(big.Int).Neg(huge)},
	} {
		t.Run(name, func(t *testing.T) { readByFastcbor[scalars](t, marshal(t, in)) })
	}
}

type small struct {
	N uint64
	S string
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
		t.Run(name, func(t *testing.T) { readByFastcbor[holder](t, marshal(t, in)) })
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
		t.Run(name, func(t *testing.T) { readByFastcbor[maps](t, data) })
	}

	// What fxamacker rejects it rejects, and what fastcbor leaves to it, it reads.
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
		// fxamacker keeps the previous key for a null one, which is not worth copying, so it decides.
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
	t.Run("a block's indexes are read by fastcbor", func(t *testing.T) {
		readByFastcbor[core.BlockTransactionsIndexes](t, enc(core.BlockTransactionsIndexes{
			Transactions: []int{0, 3}, Receipts: []int{1},
		}))
	})
}
