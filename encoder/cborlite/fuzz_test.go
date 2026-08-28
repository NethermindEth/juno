package cborlite_test

import (
	"math"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/require"
)

type reader struct {
	name string
	read func(data []byte) (int, bool)
}

func readers() []reader {
	return []reader{
		{"Head", func(data []byte) (int, bool) {
			_, _, consumed, ok := cborlite.Head(data)
			return consumed, ok
		}},
		{"Skip", cborlite.Skip},
		{"BytesNoCopy", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.BytesNoCopy(data)
			return consumed, ok
		}},
		{"Bytes", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.Bytes(data)
			return consumed, ok
		}},
		{"StringNoCopy", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.StringNoCopy(data)
			return consumed, ok
		}},
		{"String", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.String(data)
			return consumed, ok
		}},
		{"Uint64", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.Uint64(data)
			return consumed, ok
		}},
		{"Int64", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.Int64(data)
			return consumed, ok
		}},
		{"BigInt", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.BigInt(data)
			return consumed, ok
		}},
		{"Bool", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.Bool(data)
			return consumed, ok
		}},
		{"Tag", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.Tag(data)
			return consumed, ok
		}},
		{"ArrayHeader", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.ArrayHeader(data)
			return consumed, ok
		}},
		{"MapHeader", func(data []byte) (int, bool) {
			_, consumed, ok := cborlite.MapHeader(data)
			return consumed, ok
		}},
		{"ReadNull", cborlite.ReadNull},
	}
}

func FuzzReadersStayInBounds(f *testing.F) {
	f.Add([]byte{})
	f.Add(head(uintMajor, 0))
	f.Add([]byte{null})
	f.Add([]byte{simpleFalse})
	f.Add([]byte{simpleTrue})
	f.Add(head(simpleMajor, 23)) // undefined, which is neither null nor a boolean
	f.Add(head(simpleMajor, 32))
	f.Add(head(uintMajor, 255))
	f.Add(head(uintMajor, math.MaxUint64))
	f.Add(cborBytes(0xaa, 0xbb, 0xcc))
	f.Add(cborText("abc"))
	f.Add(cborArray(head(uintMajor, 1), head(uintMajor, 2)))
	f.Add(cborMap(cborText("k"), head(uintMajor, 1)))
	f.Add(cborTagged(tagPositiveBignum, cborBytes(beyondUint64.Bytes()...)))
	f.Add(cborArray(
		cborArray(head(uintMajor, 1), head(uintMajor, 2)),
		cborMap(head(uintMajor, 3), head(uintMajor, 4)),
	))
	f.Add([]byte{initialByte(arrayMajor, 31), 1, 0xff})          // Indefinite length.
	f.Add([]byte{initialByte(arrayMajor, info1Byte), 255, 0x01}) // Too little bytes.

	all := readers()

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1024 {
			return
		}

		points := []int{0, 1, len(data) / 2, len(data)}

		for _, r := range all {
			for _, at := range points {
				if at > len(data) {
					continue
				}

				window := data[at:]
				consumed, ok := r.read(window)
				if !ok {
					continue
				}

				require.Positivef(t, consumed,
					"%s reported success without making progress on %d bytes",
					r.name, len(window))
				require.LessOrEqualf(t, consumed, len(window),
					"%s reported consuming %d of %d bytes",
					r.name, consumed, len(window))
			}
		}
	})
}
