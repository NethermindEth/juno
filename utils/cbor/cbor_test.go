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

type shape interface{ sides() int }

type square struct{ Side uint64 }

func (*square) sides() int { return 4 }

type circle struct{ Radius uint64 }

func (*circle) sides() int { return 0 }

func TestRegisteredInterface(t *testing.T) {
	require.NoError(t, cbor.RegisterType(reflect.TypeFor[square]()))
	require.NoError(t, cbor.RegisterType(reflect.TypeFor[circle]()))
	var contents [][]byte
	cbor.RegisterDecoder(reflect.TypeFor[square](), func(data []byte, v any) error {
		contents = append(contents, data)
		out := reflect.ValueOf(v).Elem()
		if out.Kind() == reflect.Pointer {
			out.Set(reflect.New(out.Type().Elem()))
			out = out.Elem()
		}
		out.Set(reflect.ValueOf(square{Side: 7}))
		return nil
	})
	cbor.RegisterInterface(reflect.TypeFor[shape]())

	tagged, err := cbor.Marshal(&square{Side: 1})
	require.NoError(t, err)
	untagged, err := cbor.Marshal(struct{ Side uint64 }{Side: 1})
	require.NoError(t, err)
	require.Greater(t, len(tagged), len(untagged))
	content := tagged[len(tagged)-len(untagged):]

	t.Run("the tag selects the type, whose decoder reads the content", func(t *testing.T) {
		contents = nil
		var value shape
		require.NoError(t, cbor.Unmarshal(tagged, &value))
		require.Equal(t, &square{Side: 7}, value)

		var pointer *shape
		require.NoError(t, cbor.Unmarshal(tagged, &pointer))
		require.Equal(t, &square{Side: 7}, *pointer)
		require.Equal(t, [][]byte{content, content}, contents)
	})

	t.Run("reads of a tagged type require its tag", func(t *testing.T) {
		contents = nil
		var value *square
		require.NoError(t, cbor.Unmarshal(tagged, &value))
		require.Equal(t, &square{Side: 7}, value)
		require.Equal(t, [][]byte{content}, contents)

		value = nil
		require.Error(t, cbor.Unmarshal(untagged, &value), "fxamacker requires the tag too")
		require.Len(t, contents, 1)
	})

	t.Run("types without a decoder use fxamacker", func(t *testing.T) {
		contents = nil
		data, err := cbor.Marshal(&circle{Radius: 2})
		require.NoError(t, err)
		var value shape
		require.NoError(t, cbor.Unmarshal(data, &value))
		require.Equal(t, &circle{Radius: 2}, value)
		require.Empty(t, contents)
	})

	t.Run("untagged, unknown and truncated tags use fxamacker", func(t *testing.T) {
		contents = nil
		unknown := append([]byte{0xd9, 0x01, 0x00}, untagged...) // tag 256
		for _, data := range [][]byte{untagged, unknown, tagged[:3], tagged[:5]} {
			var value shape
			require.Error(t, cbor.Unmarshal(data, &value))
			require.Nil(t, value)
		}
		require.Empty(t, contents)
	})
}
