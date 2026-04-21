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

func benchJSONInputs(b *testing.B) []struct {
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
	for _, tc := range benchJSONInputs(b) {
		f := felt.UnsafeFromString[felt.Felt](tc.hex)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var out []byte
			for b.Loop() {
				out, _ = f.MarshalJSON()
			}
			benchBytesSink = out
		})
	}
}

func BenchmarkUnmarshalJSON(b *testing.B) {
	for _, tc := range benchJSONInputs(b) {
		input := []byte(`"` + tc.hex + `"`)
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var f felt.Felt
			for b.Loop() {
				_ = f.UnmarshalJSON(input)
			}
			benchFeltSink = f
		})
	}
}
