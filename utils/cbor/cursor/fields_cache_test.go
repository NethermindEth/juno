package cursor_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

// Bytes after a key must be masked out, while every byte inside the key must
// participate in matching. Sibling names also exercise a cached miss followed
// by a later table entry, and case variants exercise alternate name matching.
func TestFieldsCachedComparisons(t *testing.T) {
	for _, length := range []int{0, 1, 6, 7, 8, 14, 15, 16, 22, 23, 24, 25, 31, 32} {
		t.Run(fmt.Sprintf("length%d", length), func(t *testing.T) {
			base := strings.Repeat("a", length)
			names := []string{base, base + "x", "b" + base}
			fields := cursor.NewFields(names...)
			for target, name := range names {
				variants := []struct {
					key  []byte
					want int
				}{{[]byte(name), target}, {[]byte(strings.ToUpper(name)), target}}
				for pos := range len(name) {
					key := []byte(name)
					key[pos] = 'z'
					variants = append(variants, struct {
						key  []byte
						want int
					}{key, cursor.Unknown})
				}
				for _, variant := range variants {
					for _, value := range [][]byte{nil, bytes.Repeat([]byte{0}, 7), bytes.Repeat([]byte{0xff}, 23),
						bytes.Repeat([]byte{0}, 24), bytes.Repeat([]byte{0x7f}, 24),
						bytes.Repeat([]byte{0x80}, 24), bytes.Repeat([]byte{0xff}, 24)} {
						width := byte(0)
						if len(variant.key) >= 24 {
							width = 1
						}
						input := appendTestCBORArgument([]byte{0, 1, 2, 0xa1}, 0x60, uint64(len(variant.key)), width)
						input = append(input, variant.key...)
						valueOffset := len(input)
						valueWidth := byte(0)
						if len(value) >= 24 {
							valueWidth = 1
						}
						input = appendTestCBORArgument(input, 0x40, uint64(len(value)), valueWidth)
						input = append(input, value...)
						before := bytes.Clone(input)
						r := openSkipCursor(t, input, 0)
						s, err := r.OpenStruct(fields)
						require.NoError(t, err)
						field, err := r.NextField(&s)
						require.NoError(t, err)
						require.Equal(t, variant.want, field, "key %q, value %x", variant.key, value)
						require.Equal(t, valueOffset, r.Pos())
						got, err := r.BytesBorrowed()
						require.NoError(t, err)
						require.True(t, bytes.Equal(value, got))
						field, err = r.NextField(&s)
						require.NoError(t, err)
						require.Equal(t, cursor.Done, field)
						require.Zero(t, r.Len())
						require.Equal(t, before, input)
					}
				}
			}
		})
	}
}
