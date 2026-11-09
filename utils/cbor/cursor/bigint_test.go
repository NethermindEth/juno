package cursor_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestBigIntCarryAndOwnership(t *testing.T) {
	for _, width := range []int{0, 1, 7, 8, 9, 15, 16, 17, 31, 32, 33, 64} {
		for _, fill := range []byte{0, 1, 0x80, 0xff} {
			for _, leading := range []int{0, 3} {
				magnitude := append(make([]byte, leading), bytes.Repeat([]byte{fill}, width)...)
				for _, negative := range []bool{false, true} {
					data := []byte{0xc2}
					if negative {
						data[0] = 0xc3
					}
					if len(magnitude) < 24 {
						data = append(data, 0x40|byte(len(magnitude)))
					} else {
						data = append(data, 0x58, byte(len(magnitude)))
					}
					data = append(data, magnitude...)
					data = append(data, 7)
					before := bytes.Clone(data)
					want := new(big.Int).SetBytes(magnitude)
					if negative {
						want.Not(want)
					}
					c := cursor.NewCursor(data)
					got, err := c.BigInt()
					require.NoError(t, err)
					require.Equal(t, want, got)
					require.Equal(t, before, data)
					require.Equal(t, []byte{7}, c.Remaining())
					for i := range data {
						data[i] ^= 0xff
					}
					require.Equal(t, want, got, "result borrows its magnitude")
				}
			}
		}
	}
	for _, head := range []byte{0x1b, 0x3b} {
		for _, low := range []byte{0xfe, 0xff} {
			data := []byte{head, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, low}
			want := new(big.Int).SetBytes(data[1:])
			if head == 0x3b {
				want.Not(want)
			}
			c := cursor.NewCursor(data)
			got, err := c.BigInt()
			require.NoError(t, err)
			require.Equal(t, want, got)
			require.Empty(t, c.Remaining())
		}
	}
}
