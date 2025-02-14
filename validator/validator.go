package validator

import (
	"log"
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	"github.com/go-playground/validator/v10"
)

var (
	once sync.Once
	v    *validator.Validate
)

func validateResourceBounds(fl validator.FieldLevel) bool {
	req, ok := fl.Parent().Interface().(rpcv8.Transaction)
	if !ok {
		return false
	}

	if (req.Version.String() == "0x3" || req.Version.String() == "0x100000000000000000000000000000003") &&
		(req.ResourceBounds == nil || len(*req.ResourceBounds) != 3) {
		return false
	}
	return true
}

// Validator returns a singleton that can be used to validate various objects
func Validator() *validator.Validate {
	once.Do(func() {
		v = validator.New()

		if err := v.RegisterValidation("resource_bounds_required", validateResourceBounds); err != nil {
			log.Fatalf("failed to register validation: %v", err)
		}

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
			if t, ok := field.Interface().(rpcv6.TransactionType); ok {
				return t.String()
			}
			panic("not an rpc v6 TransactionType")
		}, rpcv6.TransactionType(0))
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			if t, ok := field.Interface().(rpcv7.TransactionType); ok {
				return t.String()
			}
			panic("not an rpc v7 TransactionType")
		}, rpcv7.TransactionType(0))
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			if t, ok := field.Interface().(rpcv8.TransactionType); ok {
				return t.String()
			}
			panic("not an rpc v8 TransactionType")
		}, rpcv8.TransactionType(0))
	})
	return v
}
