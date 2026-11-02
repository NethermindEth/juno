package cbor_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// TestFeltSliceCountsAsALevel: a slice of felts is an array, one nesting level to fxamacker, and a
// felt in it is another. A felt with more items than four skips them, so how deep they nest decides
// whether fxamacker rejects the slice, and the reader must reject exactly the same slices.
func TestFeltSliceCountsAsALevel(t *testing.T) {
	for nested := 26; nested <= 34; nested++ {
		// One felt of five items, the fifth nested arrays of one item each.
		item := []byte{0x81, 0x85, 0, 0, 0, 0}
		item = append(item, bytes.Repeat([]byte{0x81}, nested)...)
		item = append(item, 0)

		var want [][cursor.Limbs]uint64
		_, fxErr := fxamacker.UnmarshalFirst(item, &want)
		c := cursor.NewCursor(item)
		_, err := cursor.LimbSlice[[cursor.Limbs]uint64](&c)
		require.Equalf(t, fxErr == nil, err == nil,
			"%d levels: fxamacker %v, reader %v", nested, fxErr, err)
	}
}
