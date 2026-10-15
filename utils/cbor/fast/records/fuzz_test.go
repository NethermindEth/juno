package records_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// fuzzRouted decodes arbitrary bytes into a routed type through the wrapper (the fast reader, or
// fxamacker for what it leaves) and through fxamacker, and requires them to agree (see agree).
// The fast reader is never more lenient than fxamacker. It is stricter in one way: it reports a
// type mismatch where fxamacker coerces (a float, a bignum or a simple value for an integer, say),
// which Juno never writes; that is the one disagreement allowed.
func fuzzRouted(f *testing.F, name string) {
	kind := routedByName(name)
	for _, value := range kind.fixtures {
		data, err := cbor.Marshal(value)
		require.NoError(f, err)
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		agree(t, kind, data)
	})
}

func FuzzReceipt(f *testing.F)     { fuzzRouted(f, "receipt") }
func FuzzHeader(f *testing.F)      { fuzzRouted(f, "header") }
func FuzzTransaction(f *testing.F) { fuzzRouted(f, "transaction") }
func FuzzSierraClass(f *testing.F) { fuzzRouted(f, "class") }
func FuzzStateUpdate(f *testing.F) { fuzzRouted(f, "state") }
