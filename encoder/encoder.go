package encoder

import (
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

var (
	ts = cbor.NewTagSet()
	// https://www.iana.org/assignments/cbor-tags/cbor-tags.xhtml
	// 65536-15309735 	Unassigned
	tagNum  = uint64(65536)
	encMode cbor.EncMode
	decMode cbor.DecMode
)

func initEncModes() {
	var err error
	encMode, err = cbor.CanonicalEncOptions().EncModeWithTags(ts)
	if err != nil {
		panic(err)
	}

	decMode, err = cbor.DecOptions{}.DecModeWithTags(ts)
	if err != nil {
		panic(err)
	}
}

func RegisterType(rType reflect.Type) error {
	if err := ts.Add(
		cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
		rType,
		tagNum,
	); err != nil {
		return err
	}
	initEncModes()
	tagNum++
	return nil
}

func init() {
	initEncModes()
}

// Marshal returns encoding of param v
func Marshal(v any) ([]byte, error) {
	return encMode.Marshal(v)
}

// Unmarshal decodes param v from []byte b
func Unmarshal(b []byte, v any) error {
	return decMode.Unmarshal(b, v)
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
