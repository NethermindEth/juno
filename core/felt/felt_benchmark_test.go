package felt_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

func benchFeltInputs(b *testing.B) []struct {
	name string
	hex  string
} {
	b.Helper()
	random := felt.Random[felt.Felt]()
	return []struct {
		name string
		hex  string
	}{
		{"zero", "0x0"},
		{"small", "0xdeadbeef"},
		{"address", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"},
		{"max_felt", "0x800000000000011000000000000000000000000000000000000000000000000"},
		// Full-width (64-hex-char) input exercises the widest decode on unmarshal.
		{"padded_address", "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"},
		{"random", random.String()},
	}
}

// Measures both the encoding/json cost + the actual marshaling
func BenchmarkJSONMarshal(b *testing.B) {
	for _, tc := range benchFeltInputs(b) {
		value := felt.UnsafeFromString[felt.Felt](tc.hex)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = json.Marshal(value)
			}
		})
	}
}

func BenchmarkMarshalText(b *testing.B) {
	for _, tc := range benchFeltInputs(b) {
		value := felt.UnsafeFromString[felt.Felt](tc.hex)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = value.MarshalText()
			}
		})
	}
}

// Measures both the encoding/json cost + the actual unmarshaling
func BenchmarkJSONUnmarshal(b *testing.B) {
	for _, tc := range benchFeltInputs(b) {
		input := []byte(`"` + tc.hex + `"`)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var value felt.Felt
			for b.Loop() {
				_ = json.Unmarshal(input, &value)
			}
		})
	}
}

// Compact CBOR felts take a size set by their value, from 1 byte up to 34.
var benchCBORInputs = []struct {
	name string
	hex  string
}{
	{"zero", "0x0"},
	{"small", "0x7"},
	{"word", "0xdeadbeef"},
	{"uint64", "0xffffffffffffffff"},
	{"address", "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"},
	{"max_felt", "0x800000000000011000000000000000000000000000000000000000000000000"},
}

func BenchmarkMarshalCBOR(b *testing.B) {
	for _, tc := range benchCBORInputs {
		value := felt.UnsafeFromString[felt.Felt](tc.hex)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = value.MarshalCBOR()
			}
		})
	}
}

func BenchmarkUnmarshalCBOR(b *testing.B) {
	for _, tc := range benchCBORInputs {
		value := felt.UnsafeFromString[felt.Felt](tc.hex)
		input, err := value.MarshalCBOR()
		if err != nil {
			b.Fatal(err)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var out felt.Felt
			for b.Loop() {
				_ = out.UnmarshalCBOR(input)
			}
		})
	}
}

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
