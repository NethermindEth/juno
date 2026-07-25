package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

// Sinks prevent the compiler from optimising benchmark work away.
var (
	benchBytesSink []byte
	benchFeltSink  felt.Felt
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
		// Full-width (64-hex-char) input exercises the leading-zero-pad path on unmarshal.
		{"padded_address", "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"},
		{"random", random.String()},
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	for _, tc := range benchFeltInputs(b) {
		value := felt.UnsafeFromString[felt.Felt](tc.hex)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var out []byte
			for b.Loop() {
				out, _ = value.MarshalJSON()
			}
			benchBytesSink = out
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
			benchFeltSink = value
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
			var out []byte
			for b.Loop() {
				out, _ = value.MarshalCBOR()
			}
			benchBytesSink = out
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
			benchFeltSink = out
		})
	}
}
