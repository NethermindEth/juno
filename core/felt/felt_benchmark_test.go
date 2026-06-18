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

// BenchmarkMulLatency measures a dependent chain: each Mul waits on the previous
// result, so it is bound by the multiply's latency.
func BenchmarkMulLatency(b *testing.B) {
	c := felt.Random[felt.Felt]()
	x := felt.Random[felt.Felt]()
	for b.Loop() {
		x.Mul(&x, &c)
	}
	benchFeltSink = x
}

// BenchmarkMulThroughput runs four independent chains so the CPU can run the
// multiplies in parallel; ns/op covers four muls (divide by 4 to compare with
// BenchmarkMulLatency). The gap between the two shows how much parallelism the
// field multiply has.
func BenchmarkMulThroughput(b *testing.B) {
	c := felt.Random[felt.Felt]()
	x0 := felt.Random[felt.Felt]()
	x1 := felt.Random[felt.Felt]()
	x2 := felt.Random[felt.Felt]()
	x3 := felt.Random[felt.Felt]()
	for b.Loop() {
		x0.Mul(&x0, &c)
		x1.Mul(&x1, &c)
		x2.Mul(&x2, &c)
		x3.Mul(&x3, &c)
	}
	benchFeltSink = x0
	benchFeltSink = x1
	benchFeltSink = x2
	benchFeltSink = x3
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
