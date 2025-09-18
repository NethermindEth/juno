package types

import (
	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"golang.org/x/exp/constraints"
)

// Interface that represents all types that implement Felt as a sublying type
// E.g: Hash and Address
type FeltLike interface {
	~[4]uint64
}

// New crates a new Felt based type given any integer. Use this function with
// care since it forces a heap allocation. For efficient code use `FromUint64`
func New[F FeltLike, N constraints.Integer](num N) *F {
	var f F
	if num >= 0 {
		f = FromUint64[F](uint64(num))
	} else {
		f = FromInt64[F](int64(num))
	}
	return &f
}

// NewFromString crates a new Felt based type given a string. Use this function with
// care since it forces a heap allocation. For efficient code use `FromString`
func NewFromString[F FeltLike](val string) (*F, error) {
	f, err := FromString[F](val)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// NewFromBytes crates a new Felt based type given a byte array. Use this function with
// care since it forces a heap allocation. For efficient code use `FromBytes`
func NewFromBytes[F FeltLike](val []byte) *F {
	f := FromBytes[F](val)
	return &f
}

// NewRandom creates a new random Felt based type. It returns an error if "rand/Reader" errors
func NewRandom[F FeltLike]() (*F, error) {
	f, err := new(felt.Felt).SetRandom()
	if err != nil {
		return nil, err
	}

	ff := F(*f)
	return &ff, nil
}

// FromUint64 creates a new Felt based type given an uint64
func FromUint64[F FeltLike](num uint64) F {
	f := fp.NewElement(num)
	return F(f)
}

// FromInt64 creates a new Felt based type given an int64
func FromInt64[F FeltLike](num int64) F {
	if num >= 0 {
		return FromUint64[F](uint64(num))
	}

	// To get the negative felt value of `num` we
	// subtract Abs(num) from zero
	posNum := uint64(num * -1)
	value := FromUint64[felt.Felt](posNum)
	value.Sub(&felt.Zero, &value)
	return F(value)
}

// FromUint creates a new Felt based type given any unisgned integer
func FromUint[F FeltLike, U constraints.Unsigned](num U) F {
	return FromUint64[F](uint64(num))
}

// FromInt creates a new Felt based type given any signed integer
func FromInt[F FeltLike, S constraints.Signed](num S) F {
	return FromInt64[F](int64(num))
}

// FromBytes crates a new Felt based type given a byte array
func FromBytes[F FeltLike](value []byte) F {
	f := new(felt.Felt).SetBytes(value)
	return F(*f)
}

// FromString crates a new Felt based type given a byte array
func FromString[F FeltLike](value string) (F, error) {
	var f felt.Felt
	_, err := f.SetString(value)
	if err != nil {
		return F(f), err
	}
	return F(f), nil
}
