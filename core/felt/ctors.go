package felt

import (
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// Interface that represents all types that implement Felt as a sublying type
// E.g: Hash and Address
type FeltLike interface {
	~[4]uint64
}

// NewFromUint64 crates a new Felt based type given a uint64. Use this function with
// care since it forces a heap allocation. For efficient code use `FromUint64`
func NewFromUint64[F FeltLike](num uint64) *F {
	f := FromUint64[F](num)
	return &f
}

// NewFromString crates a new Felt based type given a string. Use this function with
// care since it forces a heap allocation. For efficient code use `FromString`
func NewFromString[F FeltLike](val string) (*F, error) {
	f, err := FromString[F](val)
	return &f, err
}

// NewUnsafeFromString is similar to `NewFromString` but it panics instead of returning
// an error.
func NewUnsafeFromString[F FeltLike](val string) *F {
	f := UnsafeFromString[F](val)
	return &f
}

// NewFromBytes crates a new Felt based type given a byte array. Use this function with
// care since it forces a heap allocation. For efficient code use `FromBytes`
func NewFromBytes[F FeltLike](val []byte) *F {
	f := FromBytes[F](val)
	return &f
}

// NewRandom creates a new random Felt based type. It returns an error if "rand/Reader" errors.
// It also forces a heap allocation. For efficient code use `Random`
func NewRandom[F FeltLike]() (*F, error) {
	f, err := Random[F]()
	return &f, err
}

// NewRandom creates a new random Felt based type. It panics if "rand/Reader" errors.
// It also forces a heap allocation. For efficient code use `UnsafeRandom`
func NewUnsafeRandom[F FeltLike]() *F {
	f := UnsafeRandom[F]()
	return &f
}

// FromUint64 creates a new Felt based type given an uint64
func FromUint64[F FeltLike](num uint64) F {
	f := fp.NewElement(num)
	return F(f)
}

// FromBytes crates a new Felt based type given a byte array
func FromBytes[F FeltLike](value []byte) F {
	f := new(Felt).SetBytes(value)
	return F(*f)
}

// FromString crates a new Felt based type given a any string
func FromString[F FeltLike](value string) (F, error) {
	f, err := new(Felt).SetString(value)
	return F(*f), err
}

// UnsafeFromString crates a new Felt based type given a any string. It panics
// if the string is not valid.
func UnsafeFromString[F FeltLike](value string) F {
	f, err := new(Felt).SetString(value)
	if err != nil {
		panic(err)
	}
	return F(*f)
}

// Creates a new random Felt based type, errors if `rand.Reader` errors
func Random[F FeltLike]() (F, error) {
	f, err := new(Felt).SetRandom()
	return F(*f), err
}

// Creates a new random Felt based type, panics if `rand.Reader` errors
func UnsafeRandom[F FeltLike]() F {
	f, err := new(Felt).SetRandom()
	if err != nil {
		panic(err)
	}
	return F(*f)
}
