package utils

import (
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func HexToFelt(t testing.TB, hex string) *felt.Felt {
	t.Helper()
	f, err := new(felt.Felt).SetString(hex)
	require.NoError(t, err)
	return f
}

func RandFelt(t *testing.T) *felt.Felt {
	t.Helper()

	f, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return f
}

func FillFelts[T any](t *testing.T, i T) T {
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	typ := v.Type()

	const feltTypeStr = "*felt.Felt"

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		ftyp := typ.Field(i).Type // Get the type of the current field

		// Skip unexported fields
		if !f.CanSet() {
			continue
		}

		switch f.Kind() {
		case reflect.Ptr:
			// Check if the type is Felt
			if ftyp.String() == feltTypeStr {
				f.Set(reflect.ValueOf(RandFelt(t)))
			} else if f.IsNil() {
				// Initialise the pointer if it's nil
				f.Set(reflect.New(ftyp.Elem()))
			}

			if f.Elem().Kind() == reflect.Struct {
				// Recursive call for nested structs
				FillFelts(t, f.Interface())
			}
		case reflect.Slice:
			// For slices, loop and populate
			for j := 0; j < f.Len(); j++ {
				elem := f.Index(j)
				if elem.Type().String() == feltTypeStr {
					elem.Set(reflect.ValueOf(RandFelt(t)))
				}
			}
		case reflect.Struct:
			// Recursive call for nested structs
			FillFelts(t, f.Addr().Interface())
		}
	}

	return i
}
