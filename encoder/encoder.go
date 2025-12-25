package encoder

import (
	"io"
	"reflect"
	"sync"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	ts = cbor.NewTagSet()
	// https://www.iana.org/assignments/cbor-tags/cbor-tags.xhtml
	// 65536-15309735 	Unassigned
	tagNum  uint64 = 65536
	encMode cbor.EncMode
	decMode cbor.DecMode
)

var initialiseEncoder sync.Once

func initEncAndDecModes() {
	var err error
	encMode, err = cbor.CanonicalEncOptions().EncModeWithTags(ts)
	if err != nil {
		panic(err)
	}

	decMode, err = cbor.DecOptions{
		MaxArrayElements: 10485760, // Set to a reasonably high value, 10MiB
	}.DecModeWithTags(ts)
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
	initEncAndDecModes()
	tagNum++
	return nil
}

// Marshal returns encoding of param v
func Marshal(v any) ([]byte, error) {
	initialiseEncoder.Do(initEncAndDecModes)
	return encMode.Marshal(v)
}

// Unmarshal decodes param v from []byte b
func Unmarshal(b []byte, v any) error {
	initialiseEncoder.Do(initEncAndDecModes)
	return decMode.Unmarshal(b, v)
}

// UnmarshalFirst decodes the first CBOR data item into param v and returns the remaining bytes
func UnmarshalFirst(b []byte, v any) ([]byte, error) {
	initialiseEncoder.Do(initEncAndDecModes)
	return decMode.UnmarshalFirst(b, v)
}

// TestSymmetry checks if a type can be marshalled and unmarshalled with no issues
func TestSymmetry(t *testing.T, value any) {
	t.Helper()
	cborBytes, err := cbor.Marshal(value)
	require.NoError(t, err)

	unmarshaled := reflect.New(reflect.TypeOf(value))
	err = cbor.Unmarshal(cborBytes, unmarshaled.Interface())
	require.NoError(t, err)
	assert.Equal(t, value, unmarshaled.Elem().Interface())
}

type Encoder interface {
	Encode(v any) error
}

// NewEncoder returns a new encoder that writes to w
func NewEncoder(w io.Writer) Encoder {
	initialiseEncoder.Do(initEncAndDecModes)
	return encMode.NewEncoder(w)
}

type Decoder interface {
	Decode(v any) error
}

// NewDecoder returns a new decoder that reads from r
func NewDecoder(r io.Reader) Decoder {
	initialiseEncoder.Do(initEncAndDecModes)
	return decMode.NewDecoder(r)
}
