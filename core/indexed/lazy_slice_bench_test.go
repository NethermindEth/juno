package indexed_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/utils/cbor/v1"
)

type benchItem struct {
	Hash  [4]uint64
	Nonce uint64
	Extra uint64
}

const benchItemsCount = 100

func newBenchLazySlice(tb testing.TB) indexed.LazySlice[benchItem] {
	tb.Helper()
	indexes := make([]int, 0, benchItemsCount)
	var data []byte
	for i := range benchItemsCount {
		encoded, err := cbor.Marshal(benchItem{
			Hash:  [4]uint64{uint64(i), uint64(i + 1), uint64(i + 2), uint64(i + 3)},
			Nonce: uint64(i),
			Extra: uint64(i * 2),
		})
		if err != nil {
			tb.Fatal(err)
		}
		indexes = append(indexes, len(data))
		data = append(data, encoded...)
	}
	return indexed.NewLazySlice[benchItem](indexes, data)
}

func BenchmarkLazySliceAll(b *testing.B) {
	lazySlice := newBenchLazySlice(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := lazySlice.All(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLazySliceIter(b *testing.B) {
	lazySlice := newBenchLazySlice(b)
	b.ReportAllocs()
	for b.Loop() {
		var sum uint64
		for item, err := range lazySlice.Iter() {
			if err != nil {
				b.Fatal(err)
			}
			sum += item.Nonce
		}
		if sum == 0 {
			b.Fatal("unexpected zero sum")
		}
	}
}

func BenchmarkLazySliceAllMapped(b *testing.B) {
	lazySlice := newBenchLazySlice(b)
	b.ReportAllocs()
	for b.Loop() {
		_, err := lazySlice.AllMapped(
			func(_ int, value benchItem) ([4]uint64, error) { return value.Hash, nil },
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLazySliceAllMappedPointerMisuse(b *testing.B) {
	lazySlice := newBenchLazySlice(b)
	b.ReportAllocs()
	for b.Loop() {
		_, err := lazySlice.AllMapped(
			func(_ int, value benchItem) (*[4]uint64, error) { return &value.Hash, nil },
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}
