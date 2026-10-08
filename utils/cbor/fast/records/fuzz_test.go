package records_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// fuzzAgainstFxamacker decodes arbitrary bytes into a fresh T through the wrapper (the fast
// reader, or fxamacker for what it leaves) and through fxamacker, and requires them to agree:
// both succeed with equal values, or both fail. The fast reader is never more lenient than
// fxamacker. It is stricter in one way: it reports a type mismatch where fxamacker coerces
// (a float, a bignum or a simple value for an integer, say), which Juno never writes; that is
// the one disagreement allowed here.
//
// skip names key text that makes an input unsafe to give fxamacker. The bloom filter package
// it calls allocates whatever bit count the bytes declare, which can take the whole process
// down; the fast reader checks that count and has its own test.
func fuzzAgainstFxamacker[T any](f *testing.F, skip string, seeds ...[]byte) {
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if skip != "" && bytes.Contains(bytes.ToLower(data), []byte(skip)) {
			t.Skip()
		}
		got, want := new(T), new(T)
		gotErr := cbor.Unmarshal(data, got)
		wantErr := fxamacker.Unmarshal(data, want)

		if wantErr == nil {
			var typeErr *fxamacker.UnmarshalTypeError
			if errors.As(gotErr, &typeErr) && typeErr.CBORType == "" {
				return // the fast reader's own type mismatch
			}
			require.NoError(t, gotErr, "fxamacker accepts what the wrapper rejects")
			require.Equal(t, want, got)
			return
		}
		require.Error(t, gotErr, "the wrapper accepts what fxamacker rejects")
	})
}

func marshal(f *testing.F, v any) []byte {
	f.Helper()
	data, err := cbor.Marshal(v)
	require.NoError(f, err)
	return data
}

func FuzzReceipt(f *testing.F) {
	fuzzAgainstFxamacker[core.TransactionReceipt](f, "", marshal(f, receipt()), marshal(f, &core.TransactionReceipt{}))
}

func FuzzHeader(f *testing.F) {
	noBloom := header()
	noBloom.EventsBloom = nil
	fuzzAgainstFxamacker[core.Header](f, "eventsbloom", marshal(f, noBloom), marshal(f, &core.Header{}))
}

func FuzzTransaction(f *testing.F) {
	var seeds [][]byte
	for _, tx := range transactions() {
		seeds = append(seeds, marshal(f, &tx))
	}
	fuzzAgainstFxamacker[core.Transaction](f, "", seeds...)
}

func FuzzSierraClass(f *testing.F) {
	class := core.ClassDefinition(sierraClass())
	fuzzAgainstFxamacker[core.ClassDefinition](f, "", marshal(f, &class))
}

func FuzzStateUpdate(f *testing.F) {
	fuzzAgainstFxamacker[core.StateUpdate](f, "", marshal(f, stateUpdate()), marshal(f, &core.StateUpdate{}))
}
