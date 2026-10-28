//go:build !compactfelt

package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

// BenchmarkUnmarshalCBORByWireSize decodes single felts across every CBOR wire shape a felt can
// take, from the 5-byte zero encoding to the 37-byte all-uint64 shape that Montgomery form makes
// near-universal in stored data.
func BenchmarkUnmarshalCBORByWireSize(b *testing.B) {
	cases := []struct {
		name    string
		limbs   [4]uint64
		wantLen int
	}{
		{"zero (5B)", [4]uint64{0, 0, 0, 0}, 5},
		{"tiny limbs (5B)", [4]uint64{1, 2, 3, 4}, 5},
		{"uint8 limbs (9B)", [4]uint64{200, 201, 202, 203}, 9},
		{"uint16 limbs (13B)", [4]uint64{40_000, 40_001, 40_002, 40_003}, 13},
		{"uint32 limbs (21B)", [4]uint64{1 << 30, 1 << 30, 1 << 30, 1 << 30}, 21},
		{"uint64 limbs (37B, canonical)", [4]uint64{1 << 40, 1 << 41, 1 << 42, 1 << 43}, 37},
		{"mixed limbs (16B)", [4]uint64{7, 1 << 40, 40_000, 200}, 16},
	}
	for _, tc := range cases {
		value := fromLimbs[felt.Felt](tc.limbs[0], tc.limbs[1], tc.limbs[2], tc.limbs[3])
		input, err := value.MarshalCBOR()
		if err != nil {
			b.Fatal(err)
		}
		if len(input) != tc.wantLen {
			b.Fatalf("%s: got %d bytes, want %d", tc.name, len(input), tc.wantLen)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var out felt.Felt
			if err := out.UnmarshalCBOR(input); err != nil {
				b.Fatal(err)
			}
			for b.Loop() {
				_ = out.UnmarshalCBOR(input)
			}
		})
	}
}
