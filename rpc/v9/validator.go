package rpcv9

import (
	"reflect"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once sync.Once
	v    *validator.Validate
)

func validateResourceBounds(fl validator.FieldLevel) bool {
	switch req := fl.Parent().Interface().(type) {
	case Transaction:
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

		// Validate TransactionType by its string representation; felts provide theirs via ValidatorValue
		v.RegisterCustomTypeFunc(func(field reflect.Value) any {
			if t, ok := field.Interface().(TransactionType); ok {
				return t.String()
			}
			panic("not an rpc v9 TransactionType")
		}, TransactionType(0))
	})
	return v
}
