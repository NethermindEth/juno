//go:build compactfelt

package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

// BenchmarkUnmarshalCBORByWireSize decodes single felts across the compact wire sizes and the
// legacy Montgomery-limb shapes that older rows still hold.
func BenchmarkUnmarshalCBORByWireSize(b *testing.B) {
	cases := []struct {
		name    string
		hex     string
		legacy  bool
		wantLen int
	}{
		{"uint (1B)", "0x7", false, 1},
		{"one byte (2B)", "0x18", false, 2},
		{"word (5B)", "0xdeadbeef", false, 5},
		{"uint64 (9B)", "0xffffffffffffffff", false, 9},
		{"hash (34B)", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", false, 34},
		{"legacy zero (5B)", "0x0", true, 5},
		{"legacy nonzero (37B)", "0xdeadbeef", true, 37},
	}
	for _, tc := range cases {
		value := felt.UnsafeFromString[felt.Felt](tc.hex)
		input, err := value.MarshalCBOR()
		if tc.legacy {
			input = legacyCBOR(b, &value)
		}
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
