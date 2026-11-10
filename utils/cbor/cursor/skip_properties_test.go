package cursor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/stretchr/testify/require"
)

func TestSkipAtomicWidths(t *testing.T) {
	limits := [...]uint64{23, 255, 65535, 1<<32 - 1, ^uint64(0)}
	for width, limit := range limits {
		for _, major := range []byte{0x00, 0x20, 0xe0} {
			for _, value := range []uint64{0, 23, 24, 32, limit} {
				if value > limit {
					continue
				}
				t.Run(fmt.Sprintf("major%x/width%d/value%d", major, width, value), func(t *testing.T) {
					wire := appendTestCBORArgument(nil, major, value, byte(width))
					var wantErr error
					if major == 0xe0 && width == 1 && value < 32 {
						wantErr = cursor.ErrSyntax
					}
					checkSkipAtomic(t, wire, wantErr, len(wire))
				})
			}
		}
		for _, major := range []byte{0x40, 0x60} {
			for _, count := range []uint64{0, 1, 23, 24, 64} {
				if count > limit {
					continue
				}
				t.Run(fmt.Sprintf("major%x/width%d/count%d", major, width, count), func(t *testing.T) {
					wire := appendTestCBORArgument(nil, major, count, byte(width))
					// Skip deliberately accepts text containing invalid UTF-8.
					wire = append(wire, bytes.Repeat([]byte{0xff}, int(count))...)
					checkSkipAtomic(t, wire, nil, 0)
				})
			}
		}
	}
}

func TestSkipAtomicRejections(t *testing.T) {
	for value := 0; value < 32; value++ {
		t.Run(fmt.Sprintf("simple%d", value), func(t *testing.T) {
			checkSkipAtomic(t, []byte{0xf8, byte(value)}, cursor.ErrSyntax, 2)
		})
	}
	for width := byte(0); width < 5; width++ {
		for tag := uint64(0); tag <= 3; tag++ {
			t.Run(fmt.Sprintf("tag%d/width%d", tag, width), func(t *testing.T) {
				wire := appendTestCBORArgument(nil, 0xc0, tag, width)
				offset := len(wire)
				wire = append(wire, bytes.Repeat([]byte{0x1b}, 40)...)
				checkSkipAtomic(t, wire, cursor.ErrUnsupported, offset)
			})
		}
		for _, major := range []byte{0x40, 0x60} {
			t.Run(fmt.Sprintf("short%x/width%d", major, width), func(t *testing.T) {
				wire := appendTestCBORArgument(nil, major, 23, width)
				offset := len(wire)
				wire = append(wire, bytes.Repeat([]byte{0xff}, 22)...)
				checkSkipAtomic(t, wire, cursor.ErrTruncated, offset)
			})
		}
	}
	for _, major := range []byte{0x40, 0x60} {
		t.Run(fmt.Sprintf("maxLength%x", major), func(t *testing.T) {
			wire := appendTestCBORArgument(nil, major, ^uint64(0), 4)
			offset := len(wire)
			wire = append(wire, bytes.Repeat([]byte{0x1b}, 40)...)
			checkSkipAtomic(t, wire, cursor.ErrTruncated, offset)
		})
	}
	t.Run("eof", func(t *testing.T) { checkSkipAtomic(t, nil, cursor.ErrTruncated, 0) })
}

// Exercise the public cursor at nonzero offsets, including the maximum enclosing
// depth. Failure is retryable and retains the diagnostic's offset within the item.
func checkSkipAtomic(t *testing.T, wire []byte, wantErr error, errorOffset int) {
	t.Helper()
	for _, depth := range []int{0, 31, 32} {
		input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
		start := len(input)
		input = append(input, wire...)
		var suffix []byte
		if wantErr == nil {
			suffix = append([]byte{0x80}, bytes.Repeat([]byte{0x1b}, 40)...)
			input = append(input, suffix...)
		}
		before := bytes.Clone(input)
		r := cursor.NewCursor(input)
		for range depth {
			n, err := r.ArrayLen()
			require.NoError(t, err)
			require.Equal(t, 1, n)
		}
		for range 3 {
			_, err := r.Uint64()
			require.NoError(t, err)
		}
		if wantErr != nil {
			for range 2 {
				err := r.Skip()
				require.ErrorIs(t, err, wantErr)
				var detail *cursor.DecodeError
				require.ErrorAs(t, err, &detail)
				require.Equal(t, start+errorOffset, detail.Offset)
				require.Equal(t, start, r.Pos())
				require.True(t, bytes.Equal(wire, r.Remaining()))
			}
		} else {
			require.NoError(t, r.Skip())
			require.Equal(t, start+len(wire), r.Pos())
			require.Equal(t, suffix, r.Remaining())
			// Skipping an atom must preserve the enclosing depth. The next
			// empty array opens one more level and detects any lost level.
			if depth == 32 {
				require.ErrorIs(t, r.Skip(), cursor.ErrDepth)
				require.Equal(t, start+len(wire), r.Pos())
			} else {
				require.NoError(t, r.Skip())
				require.Equal(t, start+len(wire)+1, r.Pos())
			}
		}
		require.Equal(t, before, input)
	}
}

func TestSkipTaggedErrors(t *testing.T) {
	leaves := []struct {
		name   string
		wire   []byte
		kind   error
		offset int
	}{
		{name: "eof", kind: cursor.ErrTruncated},
		{name: "reserved", wire: []byte{0x1c}, kind: cursor.ErrSyntax},
		{name: "break", wire: []byte{0xff}, kind: cursor.ErrSyntax},
		{name: "indefinite", wire: []byte{0x9f}, kind: cursor.ErrUnsupported},
		{name: "badTagHead", wire: []byte{0xdc}, kind: cursor.ErrSyntax},
		{name: "shortUint", wire: []byte{0x1b, 1, 2}, kind: cursor.ErrTruncated},
		{name: "shortTag", wire: []byte{0xdb, 1, 2}, kind: cursor.ErrTruncated},
		{name: "simple", wire: []byte{0xf8, 24}, kind: cursor.ErrSyntax, offset: 2},
		{name: "bytes", wire: []byte{0x42, 1}, kind: cursor.ErrTruncated, offset: 1},
		{name: "text", wire: []byte{0x62, 1}, kind: cursor.ErrTruncated, offset: 1},
		{name: "unsupportedTag", wire: []byte{0xc0}, kind: cursor.ErrUnsupported, offset: 1},
	}
	for _, depth := range []int{0, 1, 15, 31, 32} {
		for count := range 35 {
			for _, leaf := range leaves {
				t.Run(fmt.Sprintf("%s/depth%d/tags%d", leaf.name, depth, count), func(t *testing.T) {
					input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
					start := len(input)
					ends := make([]int, count)
					for i := range count {
						// Alternate preferred and non-preferred tag encodings.
						input = appendTestCBORArgument(input, 0xc0, uint64(4+i%20), byte(i%5))
						ends[i] = len(input)
					}
					wantOffset := len(input) + leaf.offset
					wantErr := leaf.kind
					input = append(input, leaf.wire...)
					// A tag before a non-tag (or EOF) is free. A tag before
					// another tag charges depth before that child's head is read.
					charged := count - 1
					if len(leaf.wire) > 0 && leaf.wire[0]&0xe0 == 0xc0 {
						charged = count
					}
					if depth+charged > 32 {
						wantErr = cursor.ErrDepth
						wantOffset = ends[32-depth]
					}
					before := bytes.Clone(input)
					r := openSkipCursor(t, input, depth)
					for range 2 {
						err := r.Skip()
						require.ErrorIs(t, err, wantErr)
						var detail *cursor.DecodeError
						require.ErrorAs(t, err, &detail)
						require.Equal(t, wantOffset, detail.Offset)
						require.Equal(t, start, r.Pos())
						require.Equal(t, input[start:], r.Remaining())
					}
					require.Equal(t, before, input)
				})
			}
		}
	}
}

func TestSkipContainerBoundaries(t *testing.T) {
	limits := [...]uint64{23, 255, 65535, 1<<32 - 1, ^uint64(0)}
	for _, major := range []byte{0x80, 0xa0} {
		perCount := uint64(1)
		if major == 0xa0 {
			perCount = 2
		}
		for width, limit := range limits {
			for _, count := range []uint64{0, 1, 4, 24, 255, 1 << 63, ^uint64(0)} {
				if count > limit {
					continue
				}
				for _, tail := range []int{0, 1, 3, 40} {
					for _, depth := range []int{0, 31, 32} {
						if depth < 32 && count <= uint64(tail)/perCount {
							continue
						}
						t.Run(fmt.Sprintf("major%x/width%d/count%d/tail%d/depth%d", major, width, count, tail, depth), func(t *testing.T) {
							input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
							start := len(input)
							input = appendTestCBORArgument(input, major, count, byte(width))
							offset := len(input)
							input = append(input, bytes.Repeat([]byte{0x1b}, tail)...)
							before := bytes.Clone(input)
							r := openSkipCursor(t, input, depth)
							kind := cursor.ErrTruncated
							if depth == 32 {
								kind = cursor.ErrDepth
							}
							for range 2 {
								err := r.Skip()
								require.ErrorIs(t, err, kind)
								var detail *cursor.DecodeError
								require.ErrorAs(t, err, &detail)
								require.Equal(t, offset, detail.Offset)
								require.Equal(t, start, r.Pos())
								require.Equal(t, input[start:], r.Remaining())
							}
							require.Equal(t, before, input)
						})
					}
				}
			}
		}
	}
}

func TestSkipContainerMalformedChildren(t *testing.T) {
	for _, major := range []byte{0x80, 0xa0} {
		for width := byte(0); width < 5; width++ {
			for prefix := range 8 {
				for _, depth := range []int{0, 30, 31} {
					for _, bad := range []byte{0x1c, 0x9f, 0xdc, 0xff} {
						t.Run(fmt.Sprintf("major%x/width%d/prefix%d/depth%d/bad%x", major, width, prefix, depth, bad), func(t *testing.T) {
							input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
							start := len(input)
							count := max(4, prefix+1)
							if major == 0xa0 {
								count = (prefix + 2) / 2
							}
							input = appendTestCBORArgument(input, major, uint64(count), width)
							for i := range prefix {
								switch {
								case depth < 31 && i%3 == 0:
									// A valid wide felt exercises the loop's own shortcut.
									input = append(input, 0x84)
									for limb := range 4 {
										input = appendTestCBORArgument(input, 0, uint64(limb+1), 4)
									}
								case depth < 31 && i%3 == 1:
									input = append(input, 0xa1, 0, 0xf6)
								default:
									input = appendTestCBORArgument(input, 0, uint64(i), 4)
								}
							}
							offset := len(input)
							input = append(input, bad)
							// Keep enough bytes for the count check and both fast windows.
							input = append(input, bytes.Repeat([]byte{0x1b}, 40)...)
							before := bytes.Clone(input)
							r := openSkipCursor(t, input, depth)
							kind := cursor.ErrSyntax
							if bad == 0x9f {
								kind = cursor.ErrUnsupported
							}
							for range 2 {
								err := r.Skip()
								require.ErrorIs(t, err, kind)
								var detail *cursor.DecodeError
								require.ErrorAs(t, err, &detail)
								require.Equal(t, offset, detail.Offset)
								require.Equal(t, start, r.Pos())
								require.Equal(t, input[start:], r.Remaining())
							}
							require.Equal(t, before, input)
						})
					}
				}
			}
		}
	}
}

func TestSkipRecursiveMalformedHeads(t *testing.T) {
	for _, depth := range []int{0, 15, 31, 32} {
		for levels := 0; levels <= 32-depth; levels++ {
			for mode := range 3 {
				for h := range 256 {
					bad := byte(h)
					if bad&31 < 28 {
						continue
					}
					t.Run(fmt.Sprintf("depth%d/levels%d/mode%d/head%x", depth, levels, mode, bad), func(t *testing.T) {
						wire := append([]byte{bad}, bytes.Repeat([]byte{0x1b}, 40)...)
						offset := 0
						for level := range levels {
							var front []byte
							width := byte(level % 5)
							switch {
							case mode == 2 && level%2 == 0:
								front = appendTestCBORArgument(nil, 0xc0, 4, width)
							case mode == 1:
								// A malformed value after a complete pair and its key.
								front = appendTestCBORArgument(nil, 0xa0, 2, width)
								front = append(front, 0, 0xf6, 1)
							default:
								// Inline-four arrays must not be mistaken for a wide felt.
								front = appendTestCBORArgument(nil, 0x80, 4, width)
								front = appendTestCBORArgument(front, 0, 7, 4)
								front = append(front, 0)
								wire = append(wire, 0)
							}
							offset += len(front)
							wire = append(front, wire...)
						}
						input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
						start := len(input)
						input = append(input, wire...)
						before := bytes.Clone(input)
						r := openSkipCursor(t, input, depth)
						kind := cursor.ErrSyntax
						if bad&31 == 31 && bad>>5 >= 2 && bad>>5 <= 5 {
							kind = cursor.ErrUnsupported
						}
						for range 2 {
							err := r.Skip()
							require.ErrorIs(t, err, kind)
							var detail *cursor.DecodeError
							require.ErrorAs(t, err, &detail)
							require.Equal(t, start+offset, detail.Offset)
							require.Equal(t, start, r.Pos())
							require.Equal(t, wire, r.Remaining())
						}
						require.Equal(t, before, input)
					})
				}
			}
		}
	}
}

func TestSkipRecursiveTruncationAndAtomicErrors(t *testing.T) {
	type rejectedInput struct {
		name   string
		wire   []byte
		kind   error
		offset int
	}
	leaves := []rejectedInput{
		{name: "eof", kind: cursor.ErrTruncated},
		{name: "simple", wire: []byte{0xf8, 24}, kind: cursor.ErrSyntax, offset: 2},
		{name: "bytes", wire: []byte{0x58, 23}, kind: cursor.ErrTruncated, offset: 2},
		{name: "text", wire: []byte{0x78, 23}, kind: cursor.ErrTruncated, offset: 2},
		{name: "tag", wire: []byte{0xc0}, kind: cursor.ErrUnsupported, offset: 1},
	}
	for major := byte(0); major < 8; major++ {
		for info := byte(24); info <= 27; info++ {
			for cut := range 1 << (info - 24) {
				leaves = append(leaves, rejectedInput{
					name: fmt.Sprintf("major%d/info%d/cut%d", major, info, cut),
					wire: append([]byte{major<<5 | info}, bytes.Repeat([]byte{0xff}, cut)...),
					kind: cursor.ErrTruncated,
				})
			}
		}
	}
	for _, depth := range []int{0, 15, 31, 32} {
		for _, levels := range []int{0, 1, 4, 16, 30, 32} {
			if depth+levels > 32 {
				continue
			}
			for _, leaf := range leaves {
				t.Run(fmt.Sprintf("%s/depth%d/levels%d", leaf.name, depth, levels), func(t *testing.T) {
					wire := bytes.Clone(leaf.wire)
					offset := leaf.offset
					for level := range levels {
						// Valid wide siblings leave enough bytes for the outer
						// count check even when its final child is absent.
						front := []byte{0xc4}
						if level%2 == 0 {
							front = append(front, 0x84)
							for i := range 3 {
								front = appendTestCBORArgument(front, 0, uint64(i+1), 4)
							}
						} else {
							front = appendTestCBORArgument(front, 0xa0, 1, byte(level%5))
							front = appendTestCBORArgument(front, 0, 7, 4)
						}
						offset += len(front)
						wire = append(front, wire...)
					}
					input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
					start := len(input)
					input = append(input, wire...)
					before := bytes.Clone(input)
					r := openSkipCursor(t, input, depth)
					for range 2 {
						err := r.Skip()
						require.ErrorIs(t, err, leaf.kind)
						var detail *cursor.DecodeError
						require.ErrorAs(t, err, &detail)
						require.Equal(t, start+offset, detail.Offset)
						require.Equal(t, start, r.Pos())
						require.True(t, bytes.Equal(wire, r.Remaining()))
					}
					require.Equal(t, before, input)
				})
			}
		}
	}
}

func TestSkipRecursiveBoundaryErrors(t *testing.T) {
	wide := []byte{0x84}
	for limb := range 4 {
		wide = appendTestCBORArgument(wide, 0, uint64(limb+1), 4)
	}
	arrayCount := appendTestCBORArgument(nil, 0x80, ^uint64(0), 4)
	mapCount := appendTestCBORArgument(nil, 0xa0, ^uint64(0), 4)
	missingTag := appendTestCBORArgument(nil, 0xc0, 4, 4)
	for _, tc := range []struct {
		name      string
		wire      []byte
		kind      error
		offset    int
		leafDepth int
	}{
		{name: "arrayCount", wire: arrayCount, kind: cursor.ErrTruncated, offset: 9, leafDepth: 31},
		{name: "mapCount", wire: mapCount, kind: cursor.ErrTruncated, offset: 9, leafDepth: 31},
		{name: "missingTag", wire: missingTag, kind: cursor.ErrTruncated, offset: 9, leafDepth: 32},
		{name: "wideAtDepth", wire: wide, kind: cursor.ErrDepth, offset: 1, leafDepth: 32},
		{name: "emptyArrayAtDepth", wire: []byte{0x80}, kind: cursor.ErrDepth, offset: 1, leafDepth: 32},
		{name: "emptyMapAtDepth", wire: []byte{0xa0}, kind: cursor.ErrDepth, offset: 1, leafDepth: 32},
		{name: "depthBeforeCount", wire: arrayCount, kind: cursor.ErrDepth, offset: 9, leafDepth: 32},
		{name: "depthBeforeShortTag", wire: []byte{0xc4, 0xdb}, kind: cursor.ErrDepth, offset: 1, leafDepth: 32},
	} {
		for _, depth := range []int{0, 15, 31} {
			t.Run(fmt.Sprintf("%s/depth%d", tc.name, depth), func(t *testing.T) {
				wire := bytes.Clone(tc.wire)
				offset := tc.offset
				for level := range tc.leafDepth - depth {
					// Each tagged container charges exactly one level. Its
					// wide unsigned sibling keeps count checks satisfied.
					front := []byte{0xc4}
					if level%2 == 0 {
						front = appendTestCBORArgument(front, 0x80, 2, byte(level%5))
					} else {
						front = appendTestCBORArgument(front, 0xa0, 1, byte(level%5))
					}
					front = appendTestCBORArgument(front, 0, 7, 4)
					offset += len(front)
					wire = append(front, wire...)
				}
				input := append(bytes.Repeat([]byte{0x81}, depth), 0, 1, 2)
				start := len(input)
				input = append(input, wire...)
				before := bytes.Clone(input)
				r := openSkipCursor(t, input, depth)
				for range 2 {
					err := r.Skip()
					require.ErrorIs(t, err, tc.kind)
					var detail *cursor.DecodeError
					require.ErrorAs(t, err, &detail)
					require.Equal(t, start+offset, detail.Offset)
					require.Equal(t, start, r.Pos())
					require.Equal(t, wire, r.Remaining())
				}
				require.Equal(t, before, input)
			})
		}
	}
}
