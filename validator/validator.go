package validator

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc"
	"github.com/go-playground/validator/v10"
)

var (
	once sync.Once
	v    *validator.Validate
)

// Validator returns a singleton that can be used to validate various objects
func Validator() *validator.Validate {
	once.Do(func() {
		v = validator.New()
		// Register these types to use their string representation for validation
		// purposes
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			switch f := field.Interface().(type) {
			case felt.Felt:
				return f.String()
			case *felt.Felt:
				return f.String()
			}
			panic("not a felt")
		}, felt.Felt{}, &felt.Felt{})
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			if t, ok := field.Interface().(rpc.TransactionType); ok {
				return t.String()
			}
			panic("not a TransactionType")
		}, rpc.TransactionType(0))
	})
	return v
}
