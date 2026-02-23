package validator

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/utils"
	"github.com/go-playground/validator/v10"
)

var (
	once sync.Once
	v    *validator.Validate
)

func validateResourceBounds(fl validator.FieldLevel) bool {
	switch req := fl.Parent().Interface().(type) {
	case rpcv8.Transaction:
		return req.ResourceBounds != nil
	case rpcv9.Transaction:
		return req.ResourceBounds != nil
	default:
		return false
	}
}

// Custom validation function for version
func validateVersion03(fl validator.FieldLevel) bool {
	version, ok := fl.Field().Interface().(string)
	return ok && (version == "0x3" || version == "0x100000000000000000000000000000003")
}

// Validator returns a singleton that can be used to validate various objects
func Validator() *validator.Validate {
	once.Do(func() {
		v = validator.New()

		if err := v.RegisterValidation("resource_bounds_required", validateResourceBounds); err != nil {
			panic("failed to register validation: " + err.Error())
		}

		if err := v.RegisterValidation("version_0x3", validateVersion03); err != nil {
			panic("failed to register validation: " + err.Error())
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
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			if t, ok := field.Interface().(rpcv9.TransactionType); ok {
				return t.String()
			}
			panic("not an rpc v9 TransactionType")
		}, rpcv9.TransactionType(0))
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			if b, ok := field.Interface().(utils.Base64); ok {
				return string(b)
			}
			panic("not a utils.Base64")
		}, utils.Base64(""))
	})
	return v
}
