package cborlite_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blob reads itself from a byte string through encoding.BinaryUnmarshaler, which is the
// interface the bloom filter of a stored header arrives on.
type blob struct{ kept []byte }

func (b *blob) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("blob: nothing to read")
	}

	// The buffer belongs to the database, so an implementation copies what it keeps.
	b.kept = append([]byte(nil), data...)
	return nil
}

func TestUnmarshalBinaryUnmarshaler(t *testing.T) {
	type holder struct{ Blob blob }

	data := cborMap(cborText("Blob"), cborBytes(0x01, 0x02, 0x03))

	var got holder
	require.NoError(t, cborlite.Unmarshal(data, &got))
	// The payload it receives is the byte string, without the header in front of it.
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, got.Blob.kept)

	t.Run("an implementation that refuses declines the decode", func(t *testing.T) {
		var got holder
		require.Error(t, cborlite.Unmarshal(cborMap(cborText("Blob"), cborBytes()), &got))
	})

	t.Run("anything but a byte string is refused", func(t *testing.T) {
		var got holder
		require.Error(t, cborlite.Unmarshal(cborMap(cborText("Blob"), cborText("no")), &got))
	})
}

/****************************************************
		Reading yourself
*****************************************************/

// pair reads itself, the way felt.Felt does, because its encoding is not something the
// generic walk can express.
type pair struct{ A, B byte }

var _ cborlite.PrefixUnmarshaler = (*pair)(nil)

func (p *pair) UnmarshalCBORPrefix(data []byte) (int, error) {
	value, next, ok := cborlite.BytesNoCopy(data)
	if !ok || len(value) != 2 {
		return 0, assert.AnError
	}

	*p = pair{A: value[0], B: value[1]}
	return next, nil
}

// liar reports consuming more than it was given, which the engine must not trust.
type liar struct{}

func (liar) UnmarshalCBORPrefix(data []byte) (int, error) { return len(data) + 1, nil }

// stallor reports success without consuming anything, which would leave a slice reading
// every element off the same bytes.
type stallor struct{}

func (stallor) UnmarshalCBORPrefix([]byte) (int, error) { return 0, nil }

func TestUnmarshalPrefixUnmarshaler(t *testing.T) {
	type holder struct {
		Pair pair
		Tail string
	}

	t.Run("reads itself, and the field after it lands", func(t *testing.T) {
		data := cborMap(
			cborText("Pair"), cborBytes(0xaa, 0xbb),
			cborText("Tail"), cborText("end"),
		)

		var got holder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, pair{A: 0xaa, B: 0xbb}, got.Pair)
		assert.Equal(t, "end", got.Tail, "a wrong consumed count would misread this")
	})

	t.Run("its error is reported under the field name", func(t *testing.T) {
		data := cborMap(cborText("Pair"), cborBytes(0xaa))

		var got holder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "Pair")
	})

	t.Run("a decoder claiming more than it was given fails", func(t *testing.T) {
		type liarHolder struct{ Value liar }

		data := cborMap(cborText("Value"), cborBytes(0xaa))

		var got liarHolder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "consuming")
	})

	t.Run("a decoder claiming nothing fails", func(t *testing.T) {
		type stallHolder struct{ Values []stallor }

		data := cborMap(cborText("Values"), cborArray(cborBytes(0xaa), cborBytes(0xbb)))

		var got stallHolder
		assert.ErrorContains(t, cborlite.Unmarshal(data, &got), "consuming")
	})

	t.Run("a pointer to one is allocated and filled", func(t *testing.T) {
		type ptrHolder struct{ Pair *pair }

		data := cborMap(cborText("Pair"), cborBytes(0xaa, 0xbb))

		var got ptrHolder
		require.NoError(t, cborlite.Unmarshal(data, &got))
		require.NotNil(t, got.Pair)
		assert.Equal(t, pair{A: 0xaa, B: 0xbb}, *got.Pair)
	})
}

func TestUnmarshalBigIntForms(t *testing.T) {
	type holder struct{ Value *big.Int }

	want := func(s string) *big.Int {
		value, ok := new(big.Int).SetString(s, 10)
		require.True(t, ok)
		return value
	}

	tests := []struct {
		name string
		data []byte
		want *big.Int
		ok   bool
	}{
		{name: "small unsigned", data: head(uintMajor, 7), want: want("7"), ok: true},
		{name: "negative", data: head(negIntMajor, 99), want: want("-100"), ok: true},
		{name: "null", data: []byte{null}, ok: true},
		{
			name: "positive bignum",
			data: cborTagged(tagPositiveBignum, cborBytes(beyondUint64.Bytes()...)),
			want: want("18446744073709551616"), ok: true,
		},
		{
			name: "negative bignum", data: cborTagged(tagNegativeBignum, cborBytes(0x01)),
			want: want("-2"), ok: true,
		},
		// The byte string is what makes this a test of the tag number. Anything else
		// after the tag is refused for not being a byte string, whatever the number.
		{name: "an unrelated tag", data: cborTagged(4, cborBytes(0x01))},
		{
			name: "a bignum tag without a byte string",
			data: cborTagged(tagPositiveBignum, head(uintMajor, 1)),
		},
		{name: "a text string", data: cborText("no")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got holder
			err := cborlite.Unmarshal(cborMap(cborText("Value"), test.data), &got)

			if !test.ok {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if test.want == nil {
				assert.Nil(t, got.Value)
				return
			}
			require.NotNil(t, got.Value)
			assert.Zerof(t, test.want.Cmp(got.Value), "want %s, got %s", test.want, got.Value)
		})
	}
}
