package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestLimbsErrorsAfterPrefix(t *testing.T) {
	faults := []struct {
		name       string
		wire       []byte
		limbError  error
		extraError error
		extraDelta int
	}{
		{"eof", nil, cursor.ErrTruncated, cursor.ErrTruncated, 0},
		{"tag", []byte{0xc0}, cursor.ErrUnsupported, cursor.ErrUnsupported, 1},
		{"simple", []byte{0xf8, 24}, cursor.ErrType, cursor.ErrSyntax, 2},
		{"bytes", []byte{0x43, 1, 2}, cursor.ErrType, cursor.ErrTruncated, 1},
		{"reserved", []byte{0x1c}, cursor.ErrSyntax, cursor.ErrSyntax, 0},
		{"break", []byte{0xff}, cursor.ErrSyntax, cursor.ErrSyntax, 0},
	}
	for width := byte(0); width < 5; width++ {
		for prefix := range 8 {
			for _, depth := range []int{0, 31} {
				check := func(t *testing.T, wire []byte, kind error, delta int) {
					t.Helper()
					input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
					start := len(input)
					input = appendTestCBORArgument(input, 0x80, uint64(prefix+1), width)
					for i := range prefix {
						if i < 4 {
							input = appendTestCBORArgument(input, 0, uint64(i+1), (width+byte(i))%5)
						} else if depth == 0 {
							// A nested map/array in the valid ignored prefix.
							input = append(input, 0xa1, 0x00, 0x81, 0xf6)
						} else {
							input = append(input, 0x42, 1, 2)
						}
					}
					badOffset := len(input)
					input = append(input, wire...)
					before := bytes.Clone(input)
					r := openSkipCursor(t, input, depth)
					old := [cursor.Limbs]uint64{9, 8, 7, 6}
					got := old
					for range 2 {
						err := r.Limbs(&got)
						require.ErrorIs(t, err, kind)
						var detail *cursor.DecodeError
						require.ErrorAs(t, err, &detail)
						require.Equal(t, badOffset+delta, detail.Offset)
						require.Equal(t, start, r.Pos())
						require.Equal(t, input[start:], r.Remaining())
						require.Equal(t, old, got)
					}
					// At depth 31 a leaked felt level would change the second
					// result to ErrDepth, making restoration observable.
					require.Equal(t, before, input)
				}
				for _, fault := range faults {
					t.Run(fmt.Sprintf("width%d/prefix%d/depth%d/%s", width, prefix, depth, fault.name), func(t *testing.T) {
						kind, delta := fault.limbError, 0
						if prefix >= 4 {
							kind, delta = fault.extraError, fault.extraDelta
						}
						check(t, fault.wire, kind, delta)
					})
				}
				for info := byte(24); info <= 27; info++ {
					for available := range 1 << (info - 24) {
						t.Run(fmt.Sprintf("width%d/prefix%d/depth%d/head%d/cut%d", width, prefix, depth, info, available), func(t *testing.T) {
							wire := append([]byte{info}, bytes.Repeat([]byte{0xff}, available)...)
							check(t, wire, cursor.ErrTruncated, 0)
						})
					}
				}
			}
		}
	}
}
