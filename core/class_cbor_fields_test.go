package core_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/require"
)

// The hand-written decoder names every field of every class type in a switch, and
// skips anything it does not recognise so an older build can still read a value a
// newer one wrote. That same skip is a trap: add a field to one of these structs
// and forget the switch, and every read silently drops it with no test failing.
//
// TestDeclaredClassFastPathCoversEveryField closes it without a list to maintain.
// It fills every field of a class through reflection, writes it, reads it back
// through the fast path, and compares. A field the decoder does not read comes
// back zero and the comparison fails.

var (
	feltType       = reflect.TypeFor[felt.Felt]()
	bigIntType     = reflect.TypeFor[*big.Int]()
	rawMessageType = reflect.TypeFor[json.RawMessage]()
	byteSliceType  = reflect.TypeFor[[]byte]()
)

// populate fills value, and everything under it, with non-zero data. It asserts as
// it goes that nothing was left zero, because a field it silently skipped would be
// a field the round-trip comparison could not catch. An unhandled type fails the
// test rather than being skipped, so a new field of a new kind is caught too.
func populate(t *testing.T, value reflect.Value, seed uint64, depth int) {
	t.Helper()

	if depth <= 0 {
		return
	}

	switch valueType := value.Type(); valueType {
	case feltType:
		value.Set(reflect.ValueOf(felt.FromUint64[felt.Felt](seed + 1)))

	case bigIntType:
		value.Set(reflect.ValueOf(big.NewInt(int64(seed) + 1)))

	case rawMessageType, byteSliceType:
		raw := []byte(fmt.Sprintf(`{"k":%d}`, seed))
		value.Set(reflect.ValueOf(raw).Convert(valueType))

	default:
		populateByKind(t, value, seed, depth)
	}
}

func populateByKind(t *testing.T, value reflect.Value, seed uint64, depth int) {
	t.Helper()

	valueType := value.Type()

	switch value.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(seed + 1)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(int64(seed) + 1)

	case reflect.String:
		value.SetString(fmt.Sprintf("value-%d", seed))

	case reflect.Bool:
		value.SetBool(true)

	case reflect.Pointer:
		value.Set(reflect.New(valueType.Elem()))
		populate(t, value.Elem(), seed, depth-1)

	case reflect.Slice:
		value.Set(reflect.MakeSlice(valueType, 1, 1))
		populate(t, value.Index(0), seed, depth-1)

	case reflect.Array:
		for index := range value.Len() {
			populate(t, value.Index(index), seed+uint64(index), depth-1)
		}

	case reflect.Struct:
		for index := range valueType.NumField() {
			field := valueType.Field(index)
			if field.PkgPath != "" { // unexported, the encoder ignores it too
				continue
			}

			populate(t, value.Field(index), seed+uint64(index), depth-1)

			// Only assert where we actually populated. At the bottom of the depth
			// budget populate returns early on purpose, to terminate on the types
			// that nest into themselves.
			if depth-1 > 1 {
				require.Falsef(t, value.Field(index).IsZero(),
					"populate left %s.%s zero, so the round-trip would not cover it",
					valueType, field.Name)
			}
		}

	default:
		require.Failf(t, "unhandled type",
			"populate does not know how to fill %s (kind %s). A new field needs a case here, "+
				"and almost certainly a case in the decoder too.", valueType, value.Kind())
	}
}

func TestDeclaredClassFastPathCoversEveryField(t *testing.T) {
	// Encoding through an alias skips MarshalBinary and yields the {At, Class} map,
	// which is the shape a real node stores.
	type plain core.DeclaredClassDefinition

	classes := []core.ClassDefinition{
		&core.SierraClass{},
		&core.DeprecatedCairoClass{},
	}

	for _, class := range classes {
		t.Run(fmt.Sprintf("%T", class), func(t *testing.T) {
			populate(t, reflect.ValueOf(class).Elem(), 1, 12)

			declared := &core.DeclaredClassDefinition{At: 7, Class: class}
			data, err := encoder.Marshal((*plain)(declared))
			require.NoError(t, err)
			require.Equal(t, byte(5), data[0]>>5, "want the map shape")

			var got core.DeclaredClassDefinition
			require.NoError(t, got.UnmarshalCBOR(data))
			require.Equal(t, declared, &got,
				"the fast path dropped a field. Add it to the matching decodeCBOR switch "+
					"in class_cbor.go.")
		})
	}
}
