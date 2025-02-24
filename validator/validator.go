package validator

import (
	"fmt"
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

	version := req.Version.String()
	fmt.Println(version)

	// Only enforce resource bounds validation for version 0x3
	if version == "0x3" || version == "0x100000000000000000000000000000003" {
		return req.ResourceBounds != nil && len(*req.ResourceBounds) == 3
	}

	// Allow earlier versions without resource bounds
	return true
}

func Validator() *validator.Validate {
	once.Do(func() {
		v = validator.New()

		// callValidationEvenIfNull is set to true to ensure that the validation is called even if the field is nil
		// If the field is nil, the validation will return false for v0-2 transactions
		if err := v.RegisterValidation("resource_bounds_required", validateResourceBounds, true); err != nil {
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
	})
	return v
}
