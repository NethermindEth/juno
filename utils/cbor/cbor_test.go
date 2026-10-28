package cbor_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

type routed struct{ Number uint64 }

type unrouted struct{ Number uint64 }

// {"Number": 1}
var numberOne = []byte{0xa1, 0x66, 'N', 'u', 'm', 'b', 'e', 'r', 1}

func TestRegisteredDecoderSelection(t *testing.T) {
	var calls int
	var fail bool
	cbor.RegisterDecoder(reflect.TypeFor[routed](), func(_ []byte, v any) error {
		calls++
		// Write through either destination shape, as a real decoder would.
		out := reflect.ValueOf(v).Elem()
		if out.Kind() == reflect.Pointer {
			out.Set(reflect.New(out.Type().Elem()))
			out = out.Elem()
		}
		out.Set(reflect.ValueOf(routed{Number: 7}))
		if fail {
			return errors.New("unsupported")
		}
		return nil
	})

	t.Run("unregistered types use fxamacker", func(t *testing.T) {
		var value unrouted
		require.NoError(t, cbor.Unmarshal(numberOne, &value))
		require.Equal(t, unrouted{Number: 1}, value)
		require.Zero(t, calls)
	})

	t.Run("zero destinations use the registered decoder", func(t *testing.T) {
		var value routed
		require.NoError(t, cbor.Unmarshal(numberOne, &value))
		require.Equal(t, routed{Number: 7}, value)

		var pointer *routed
		require.NoError(t, cbor.Unmarshal(numberOne, &pointer))
		require.Equal(t, &routed{Number: 7}, pointer)
		require.Equal(t, 2, calls)
	})

	t.Run("populated destinations use fxamacker", func(t *testing.T) {
		calls = 0
		value := routed{Number: 9}
		require.NoError(t, cbor.Unmarshal(numberOne, &value))
		require.Equal(t, routed{Number: 1}, value)
		require.Zero(t, calls)
	})

	t.Run("a failed decoder falls back from a zero destination", func(t *testing.T) {
		fail = true
		var value routed
		require.NoError(t, cbor.Unmarshal([]byte{0xa0}, &value))
		require.Equal(t, routed{}, value, "the decoder's partial result is discarded")

		var pointer *routed
		require.NoError(t, cbor.Unmarshal([]byte{0xf6}, &pointer))
		require.Nil(t, pointer)
	})

	t.Run("invalid destinations fail as in fxamacker", func(t *testing.T) {
		require.Error(t, cbor.Unmarshal(numberOne, nil))
		require.Error(t, cbor.Unmarshal(numberOne, (*routed)(nil)))
	})
}
