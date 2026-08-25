package encoder

// TODO(granza): Remove it and replace all the calls with the utils/cbor package

import (
	"io"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Encoder = cbor.Encoder

// RegisterType registers rType so it carries a tag of its own on the wire.
// It must be called only from init in encoder/registry.
func RegisterType(rType reflect.Type) error {
	return cbor.RegisterType(rType)
}

// Marshal returns encoding of param v
func Marshal(v any) ([]byte, error) {
	return cbor.Marshal(v)
}

// Unmarshal decodes param v from []byte b
func Unmarshal(b []byte, v any) error {
	return cbor.Unmarshal(b, v)
}

// UnmarshalFirst decodes the first CBOR data item into param v and returns the remaining bytes
func UnmarshalFirst(b []byte, v any) ([]byte, error) {
	return cbor.UnmarshalFirst(b, v)
}

// NewEncoder returns a new encoder
func NewEncoder(writer io.Writer) Encoder {
	return cbor.NewEncoder(writer)
}

// TestSymmetry checks if a type can be marshalled and unmarshalled with no issues
func TestSymmetry(t *testing.T, value any) {
	t.Helper()
	cborBytes, err := Marshal(value)
	require.NoError(t, err)

	unmarshaled := reflect.New(reflect.TypeOf(value))
	err = Unmarshal(cborBytes, unmarshaled.Interface())
	require.NoError(t, err)
	assert.Equal(t, value, unmarshaled.Elem().Interface())
}
