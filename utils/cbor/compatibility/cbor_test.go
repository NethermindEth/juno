package cbor_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// TestCborGoldenBytes runs the decoding contract through the cbor package, which reads the
// types that have a fast reader with it and the rest with fxamacker. Encoding is not run again:
// cbor.Marshal is fxamacker's.
func TestCborGoldenBytes(t *testing.T) {
	assertDecodesFromGolden(t, cbor.Unmarshal, goldenCases())
}

// TestCborGoldenBytesThroughInterfaces decodes the stored transactions and classes into the
// interfaces the database code reads them through, where the type is named by a tag.
func TestCborGoldenBytesThroughInterfaces(t *testing.T) {
	golden := goldenBytes(t)
	read := func(t *testing.T, name string, value any, into any) {
		t.Helper()
		stored, err := hex.DecodeString(golden[name])
		require.NoError(t, err)
		require.NoError(t, cbor.Unmarshal(stored, into))

		want := reflect.New(reflect.TypeOf(value))
		want.Elem().Set(reflect.ValueOf(value))
		require.Equal(t, want.Interface(), reflect.ValueOf(into).Elem().Interface())
	}
	for _, c := range goldenCases() {
		pointer := reflect.New(reflect.TypeOf(c.value)).Interface()
		switch pointer.(type) {
		case core.Transaction:
			t.Run(c.name, func(t *testing.T) { read(t, c.name, c.value, new(core.Transaction)) })
		case core.ClassDefinition:
			t.Run(c.name, func(t *testing.T) { read(t, c.name, c.value, new(core.ClassDefinition)) })
		}
	}
}
