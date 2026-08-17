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

func BenchmarkUnmarshalJSON(b *testing.B) {
	for _, tc := range benchFeltInputs(b) {
		input := []byte(`"` + tc.hex + `"`)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var value felt.Felt
			for b.Loop() {
				_ = value.UnmarshalJSON(input)
			}
		})
	}
}

// Felts used in CBOR pass through a Montgomery transformation,
// so they only come in two flavors: zero or nonzero.
var benchCBORInputs = []struct {
	name string
	hex  string
}{
	{"zero", "0x0"},
	{"nonzero", "0xdeadbeef"},
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

// BenchmarkUnmarshalCBORByWireSize decodes single felts across every CBOR wire shape a felt can
// take, from the 5-byte zero encoding to the 37-byte all-uint64 shape that Montgomery form makes
// near-universal in stored data.
func BenchmarkUnmarshalCBORByWireSize(b *testing.B) {
	cases := []struct {
		name  string
		limbs [4]uint64
	}{
		{"zero (5B)", [4]uint64{0, 0, 0, 0}},
		{"tiny limbs (5B)", [4]uint64{1, 2, 3, 4}},
		{"uint8 limbs (9B)", [4]uint64{200, 201, 202, 203}},
		{"uint16 limbs (13B)", [4]uint64{40_000, 40_001, 40_002, 40_003}},
		{"uint32 limbs (21B)", [4]uint64{1 << 30, 1 << 30, 1 << 30, 1 << 30}},
		{"uint64 limbs (37B, canonical)", [4]uint64{1 << 40, 1 << 41, 1 << 42, 1 << 43}},
		// Worst case for the fixed-shape fast path: three markers match before the last one
		// fails, so it pays all four compares and then the general loop from scratch.
		{"near miss (3 uint64 + tiny limb)", [4]uint64{1 << 40, 1 << 41, 1 << 42, 7}},
		{"mixed limbs (tiny first)", [4]uint64{7, 1 << 40, 40_000, 200}},
	}
	for _, tc := range cases {
		value := fromLimbs[felt.Felt](tc.limbs[0], tc.limbs[1], tc.limbs[2], tc.limbs[3])
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
