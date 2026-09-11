package felt_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/v1"
)

func BenchmarkSliceVsFeltArrayCBOR(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		slice := randomSlice[feltoid](n)
		feltArray := []feltoid(slice)
		encoded, err := cbor.Marshal(slice)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("marshal/Slice/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = cbor.Marshal(slice)
			}
		})
		b.Run(fmt.Sprintf("marshal/FeltArray/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = cbor.Marshal(feltArray)
			}
		})
		b.Run(fmt.Sprintf("unmarshal/Slice/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var out felt.Slice[feltoid]
				_ = cbor.Unmarshal(encoded, &out)
			}
		})
		b.Run(fmt.Sprintf("unmarshal/FeltArray/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var out []felt.Felt
				_ = cbor.Unmarshal(encoded, &out)
			}
		})
	}
}

func shortSlice(size int) felt.Slice[felt.Felt] {
	slice := make(felt.Slice[felt.Felt], size)
	for idx := range slice {
		slice[idx] = felt.FromUint64[felt.Felt](uint64(idx) % 4096)
	}

	return slice
}

func BenchmarkSliceVsFeltArrayJSON(b *testing.B) {
	cases := map[string]func(int) felt.Slice[felt.Felt]{
		"short": shortSlice,
		"long":  randomSlice[felt.Felt],
	}

	for _, name := range []string{"short", "long"} {
		for _, n := range []int{1000, 5000} {
			slice := cases[name](n)
			feltArray := []felt.Felt(slice)

			encoded, err := json.Marshal(slice)
			if err != nil {
				b.Fatal(err)
			}

			b.Run(fmt.Sprintf("marshal/Slice/%s/n=%d", name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := json.Marshal(slice); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(fmt.Sprintf("marshal/FeltArray/%s/n=%d", name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := json.Marshal(feltArray); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(fmt.Sprintf("unmarshal/Slice/%s/n=%d", name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var out felt.Slice[felt.Felt]
					if err := json.Unmarshal(encoded, &out); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run(fmt.Sprintf("unmarshal/FeltArray/%s/n=%d", name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var out []felt.Felt
					if err := json.Unmarshal(encoded, &out); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
