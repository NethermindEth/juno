package cborlite_test

import (
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnmarshalPrefix covers the entry point a self-reading type is built on: it stops at
// the end of the value and reports where that was, instead of demanding the whole buffer.
func TestUnmarshalPrefix(t *testing.T) {
	type holder struct{ Value uint64 }

	value := cborMap(cborText("Value"), []byte{0x07})
	data := append(append([]byte{}, value...), 0xff, 0xff)

	var got holder
	consumed, err := cborlite.UnmarshalPrefix(data, &got)
	require.NoError(t, err)
	assert.Equal(t, len(value), consumed)
	assert.Equal(t, uint64(7), got.Value)

	t.Run("the same bytes fail Unmarshal, which wants all of them", func(t *testing.T) {
		var got holder
		err := cborlite.Unmarshal(data, &got)
		assert.ErrorContains(t, err, "left over")
	})
}

// TestUnmarshalStrict covers the difference from [cborlite.Unmarshal]: a key no field
// reads is an error rather than something to skip.
func TestUnmarshalStrict(t *testing.T) {
	type both struct {
		First  uint64
		Second uint64
	}
	type onlyFirst struct{ First uint64 }

	data := cborMap(cborText("First"), []byte{0x01}, cborText("Second"), []byte{0x02})

	var complete both
	require.NoError(t, cborlite.UnmarshalStrict(data, &complete))
	assert.Equal(t, both{First: 1, Second: 2}, complete)

	t.Run("a struct that leaves a field out fails, and names the key", func(t *testing.T) {
		var got onlyFirst
		err := cborlite.UnmarshalStrict(data, &got)
		require.Error(t, err)
		assert.ErrorContains(t, err, "Second")
	})

	t.Run("the same struct passes without strict", func(t *testing.T) {
		var got onlyFirst
		require.NoError(t, cborlite.Unmarshal(data, &got))
		assert.Equal(t, uint64(1), got.First)
	})
}
