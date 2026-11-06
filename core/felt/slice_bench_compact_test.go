//go:build compactfelt

package felt_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
)

// BenchmarkSliceCBOR compares compact and legacy slice decoding on small and hash-sized values.
func BenchmarkSliceCBOR(b *testing.B) {
	cases := map[string]func(int) felt.Slice[felt.Felt]{
		"short": shortSlice,
		"long":  randomSlice[felt.Felt],
	}

	for _, name := range []string{"short", "long"} {
		const n = 5000
		slice := cases[name](n)
		compact, err := slice.MarshalCBOR()
		if err != nil {
			b.Fatal(err)
		}
		// The same slice as the previous encoder wrote it: an array header of n items, each a
		// Montgomery-limb array.
		legacy := binary.BigEndian.AppendUint16([]byte{0x99}, n) // array header, uint16 item count
		for i := range slice {
			legacy = append(legacy, legacyCBOR(b, &slice[i])...)
		}

		b.Run(fmt.Sprintf("marshal/%s/n=%d", name, n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = slice.MarshalCBOR()
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/felt")
		})
		for _, input := range []struct {
			format string
			data   []byte
		}{{"compact", compact}, {"legacy", legacy}} {
			b.Run(fmt.Sprintf("unmarshal/%s/%s/n=%d", input.format, name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var out felt.Slice[felt.Felt]
					if err := out.UnmarshalCBOR(input.data); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/felt")
				b.ReportMetric(float64(len(input.data))/n, "B/felt")
			})
			// The generic decoder calls UnmarshalCBOR per element, as struct fields are decoded.
			b.Run(fmt.Sprintf("unmarshal-generic/%s/%s/n=%d", input.format, name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var out []felt.Felt
					if err := cbor.Unmarshal(input.data, &out); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/felt")
				b.ReportMetric(float64(len(input.data))/n, "B/felt")
			})
		}
	}
}
