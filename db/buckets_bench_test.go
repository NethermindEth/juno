package db_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
)

// keySink keeps Key's result escaping so the allocation is measured faithfully.
// otherwise compiler optimizations will give us better results for the empty
// key tests
var keySink []byte

func BenchmarkKey(b *testing.B) {
	felt := make([]byte, 32)
	prefix := []byte{0xab}

	manyFelts := func() [][]byte {
		res := make([][]byte, 1000)
		for i := range res {
			res[i] = felt
		}
		return res
	}

	// Cases mirror real call sites: most keys are empty (prefix only) or a
	// single 32-byte felt; composite keys and wider fan-outs are the stress cases.
	cases := []struct {
		name string
		keys [][]byte
	}{
		{"NoKey", nil},
		{"SingleFelt", [][]byte{felt}},
		{"PrefixPlusFelt", [][]byte{prefix, felt}},
		{"TwoFelts", [][]byte{felt, felt}},
		{"FourFelts", [][]byte{felt, felt, felt, felt}},
		{"ManyFelts", manyFelts()},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				keySink = db.StateTrie.Key(c.keys...)
			}
		})
	}
}
