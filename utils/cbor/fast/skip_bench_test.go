package fast_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// feltArray is n felts in the 37 byte form, as a Sierra program is stored.
func feltArray(n int) []byte {
	felt := []byte{0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8}
	data := []byte{0x9a, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	for range n {
		data = append(data, felt...)
	}
	return data
}

func BenchmarkSkipFeltArray(b *testing.B) {
	data := feltArray(10000)
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		r := fast.NewReader(data)
		if err := r.Skip(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadFeltArray(b *testing.B) {
	data := feltArray(10000)
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		r := fast.NewReader(data)
		if _, err := fast.LimbSlice[[4]uint64](&r); err != nil {
			b.Fatal(err)
		}
	}
}

// mixedFeltArray is n felts where one in sixteen has short limbs, roughly the share of
// small values in compiled class bytecode.
func mixedFeltArray(n int) []byte {
	long := []byte{0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8}
	short := []byte{0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x19, 1, 2, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x00}
	data := []byte{0x9a, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	for i := range n {
		if i%16 == 0 {
			data = append(data, short...)
		} else {
			data = append(data, long...)
		}
	}
	return data
}

func BenchmarkReadMixedFeltArray(b *testing.B) {
	data := mixedFeltArray(10000)
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		r := fast.NewReader(data)
		if _, err := fast.LimbSlice[[4]uint64](&r); err != nil {
			b.Fatal(err)
		}
	}
}
