package encoder

import (
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

// Marshal returns encoding of param v
func Marshal(v any) ([]byte, error) {
	return cbor.Marshal(v)
}

// Unmarshal decodes param v from []byte b
func Unmarshal(b []byte, v any) error {
	return cbor.Unmarshal(b, v)
}

// TestSymmetry checks if a type can be marshaled and unmarshalled with no issues
func TestSymmetry(t *testing.T, value any) {
	cborBytes, err := cbor.Marshal(value)
	assert.NoError(t, err)

	unmarshaled := reflect.New(reflect.TypeOf(value))
	err = cbor.Unmarshal(cborBytes, unmarshaled.Interface())
	assert.NoError(t, err)
	assert.Equal(t, value, unmarshaled.Elem().Interface())
}
