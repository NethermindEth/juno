package cborlite_test

import (
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/require"
)

// The package promises that a truncated or corrupt buffer makes a reader return
// ok == false rather than panicking or reading out of range. Table tests can only
// cover the shapes someone thought of, so the promise is checked here as a property
// over arbitrary bytes.
//
// The property is stronger than "does not panic". Every reader that reports success
// must also have made progress and stayed inside the buffer:
//
//	offset < next <= len(data)
//
// Without the left half a caller looping over a map or array would spin forever on
// a reader that returns the offset it was given. Without the right half it would
// read out of range on the next item.

// reader is one of the package's readers, reduced to the offset it reports.
type reader struct {
	name string
	read func(data []byte, offset int) (int, bool)
}

func readers() []reader {
	skipValue := func(_ *struct{}, data []byte, offset int) (int, bool) {
		return cborlite.Skip(data, offset)
	}

	return []reader{
		{"Head", func(data []byte, offset int) (int, bool) {
			_, _, next, ok := cborlite.Head(data, offset)
			return next, ok
		}},
		{"Skip", cborlite.Skip},
		{"ByteString", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.ByteString(data, offset)
			return next, ok
		}},
		{"ByteStringCopy", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.ByteStringCopy(data, offset)
			return next, ok
		}},
		{"TextString", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.TextString(data, offset)
			return next, ok
		}},
		{"String", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.String(data, offset)
			return next, ok
		}},
		{"Uint64", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.Uint64(data, offset)
			return next, ok
		}},
		{"BigInt", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.BigInt(data, offset)
			return next, ok
		}},
		{"ArrayHeader", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.ArrayHeader(data, offset)
			return next, ok
		}},
		{"MapHeader", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.MapHeader(data, offset)
			return next, ok
		}},
		{"StringSlice", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.StringSlice(data, offset)
			return next, ok
		}},
		{"StructSlice", func(data []byte, offset int) (int, bool) {
			_, next, ok := cborlite.StructSlice(data, offset, skipValue)
			return next, ok
		}},
		{"StructMap", func(data []byte, offset int) (int, bool) {
			return cborlite.StructMap(data, offset,
				func(_, data []byte, offset int) (int, bool) {
					return cborlite.Skip(data, offset)
				})
		}},
		{"ReadNull", cborlite.ReadNull},
	}
}

func FuzzReadersStayInBounds(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{cborlite.Null})
	f.Add([]byte{0x18, 0xff})
	f.Add([]byte{0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	f.Add([]byte{0x43, 0xaa, 0xbb, 0xcc})
	f.Add([]byte{0x63, 'a', 'b', 'c'})
	f.Add([]byte{0x82, 0x01, 0x02})
	f.Add([]byte{0xa1, 0x61, 'k', 0x01})
	f.Add([]byte{0xc2, 0x49, 0x01, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{0x82, 0x82, 0x01, 0x02, 0xa1, 0x03, 0x04})
	f.Add([]byte{0x9f, 0x01, 0xff}) // indefinite length, which the readers reject
	f.Add([]byte{0x98, 0xff, 0x01}) // a count far past the remaining bytes

	all := readers()

	f.Fuzz(func(t *testing.T, data []byte) {
		// Structural variety finds reader bugs, size does not. And size is actively
		// harmful here: a valid array of a million one-byte elements really does
		// build a million-element slice, and this runs every reader at every offset,
		// so one large input costs seconds and starves the fuzzer of executions.
		if len(data) > 1024 {
			return
		}

		// Offsets around the edges matter as much as the bytes: the readers have to
		// hold up at the end of the buffer and past it.
		offsets := []int{-1, 0, 1, len(data) - 1, len(data), len(data) + 1}

		for _, r := range all {
			for _, offset := range offsets {
				next, ok := r.read(data, offset)
				if !ok {
					continue
				}

				require.Greaterf(t, next, offset,
					"%s reported success without making progress at offset %d of %d bytes",
					r.name, offset, len(data))
				require.LessOrEqualf(t, next, len(data),
					"%s reported an offset past the buffer at offset %d of %d bytes",
					r.name, offset, len(data))
			}
		}
	})
}
