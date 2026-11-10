package cursor_test

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestContainerErrorsPreserveState(t *testing.T) {
	for _, container := range []struct {
		name  string
		major byte
		read  func(*cursor.Cursor) (int, error)
	}{
		{"array", 0x80, (*cursor.Cursor).ArrayLen},
		{"map", 0xa0, (*cursor.Cursor).MapLen},
	} {
		for _, width := range []int{0, 1, 2, 4, 8} {
			for _, tooLarge := range []bool{false, true} {
				name := container.name + "/width=" + strconv.Itoa(width) + "/excess-count=" + strconv.FormatBool(tooLarge)
				t.Run(name, func(t *testing.T) {
					// At depth 32, zero is a valid count but opening another container
					// exceeds the nesting limit. A positive count with no payload must
					// report truncation instead, before checking depth.
					count := uint64(0)
					if tooLarge {
						count = 1
						if width == 8 {
							count = ^uint64(0)
						}
					}
					var head []byte
					switch width {
					case 0:
						head = []byte{container.major | byte(count)}
					case 1:
						head = []byte{container.major | 24, byte(count)}
					case 2:
						head = binary.BigEndian.AppendUint16([]byte{container.major | 25}, uint16(count))
					case 4:
						head = binary.BigEndian.AppendUint32([]byte{container.major | 26}, uint32(count))
					case 8:
						head = binary.BigEndian.AppendUint64([]byte{container.major | 27}, count)
					}
					input := append(bytes.Repeat([]byte{0x81}, 32), head...)
					before := bytes.Clone(input)
					c := cursor.NewCursor(input)
					for range 32 {
						n, err := c.ArrayLen()
						require.NoError(t, err)
						require.Equal(t, 1, n)
					}
					wantError := cursor.ErrDepth
					if tooLarge {
						wantError = cursor.ErrTruncated
					}
					for range 2 {
						n, err := container.read(&c)
						require.Zero(t, n)
						require.ErrorIs(t, err, wantError)
						var detail *cursor.DecodeError
						require.ErrorAs(t, err, &detail)
						require.Equal(t, 32, detail.Offset)
						require.Equal(t, 32, c.Pos())
						require.Equal(t, head, c.Remaining())
						require.Equal(t, before, input)
					}
					// A failed open must not change depth: closing one enclosing
					// container makes the same zero-count item readable.
					if !tooLarge {
						c.Leave()
						n, err := container.read(&c)
						require.NoError(t, err)
						require.Zero(t, n)
						require.Zero(t, c.Len())
					}
				})
			}
		}
	}
}
