package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func FuzzTransactionReads(f *testing.F) { fuzzInterfaceReads[core.Transaction](f) }

func FuzzClassReads(f *testing.F) { fuzzInterfaceReads[core.ClassDefinition](f) }

// Whenever fxamacker reads an input into the interface i, Unmarshal must
// return the same value. Other inputs must not panic.
func fuzzInterfaceReads[i any](f *testing.F) {
	golden := goldenBytes(f)
	for _, c := range goldenCases() {
		if !reflect.PointerTo(reflect.TypeOf(c.value)).Implements(reflect.TypeFor[i]()) {
			continue
		}
		stored, err := hex.DecodeString(golden[c.name])
		require.NoError(f, err)
		f.Add(stored)
		f.Add(stored[:len(stored)/2])
		f.Add(stored[5:]) // without the tag
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if mayHoldBignums(data) {
			return
		}
		var expected, actual i
		if fxamacker.Unmarshal(data, &expected) != nil {
			_ = cbor.Unmarshal(data, &actual)
			return
		}
		require.NoError(t, cbor.Unmarshal(data, &actual))
		require.Equal(t, expected, actual)
	})
}
