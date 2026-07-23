package felt_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
)

func BenchmarkSliceVsFeltArray(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		slice := randomSlice[feltoid](n)
		feltArray := []feltoid(slice)
		encoded, err := encoder.Marshal(slice)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("marshal/Slice/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = encoder.Marshal(slice)
			}
		})
		b.Run(fmt.Sprintf("marshal/FeltArray/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = encoder.Marshal(feltArray)
			}
		})
		b.Run(fmt.Sprintf("unmarshal/Slice/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var out felt.Slice[feltoid]
				_ = encoder.Unmarshal(encoded, &out)
			}
		})
		b.Run(fmt.Sprintf("unmarshal/FeltArray/n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var out []felt.Felt
				_ = encoder.Unmarshal(encoded, &out)
			}
		})
	}
}
