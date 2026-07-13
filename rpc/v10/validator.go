package rpcv10

import (
	"math/big"
	"reflect"
	"strconv"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/go-playground/validator/v10"
)

var (
	once sync.Once
	v    *validator.Validate
)

// Custom validation function for version
func validateVersion03(fl validator.FieldLevel) bool {
	version, ok := fl.Field().Interface().(string)
	return ok && (version == "0x3" || version == "0x100000000000000000000000000000003")
}

// validateFeltMaxBits checks that a felt fits within the number of bits given
// as the validation tag parameter (e.g. `felt_max_bits=64` ensures the felt is
// within the range of a uint64).
func validateFeltMaxBits(fl validator.FieldLevel) bool {
	s, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}

	maxBits, err := strconv.Atoi(fl.Param())
	if err != nil {
		return false
	}

	bigInt, ok := new(big.Int).SetString(s, 0)
	if !ok {
		return false
	}

	return bigInt.BitLen() <= maxBits
}

// Validator returns a singleton that can be used to validate various objects
func Validator() *validator.Validate {
	once.Do(func() {
		v = validator.New(validator.WithRequiredStructEnabled())

		if err := v.RegisterValidation("version_0x3", validateVersion03); err != nil {
			panic("failed to register validation: " + err.Error())
		}

		if err := v.RegisterValidation("felt_max_bits", validateFeltMaxBits); err != nil {
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
			if t, ok := field.Interface().(TransactionType); ok {
				return t.String()
			}
			panic("not an rpc v10 TransactionType")
		}, TransactionType(0))
	})
	return v
}
