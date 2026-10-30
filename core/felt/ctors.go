package felt

import (
	"unsafe"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// FeltLike is the constraint for generic code over felts: every Typed[K] satisfies it, as does any
// type declared over one. It exists because a generic type such as Slice cannot infer K.
type FeltLike interface {
	~[4]uint64
}

// Zero returns the zero value of F.
func Zero[F FeltLike]() F {
	var zero F
	return zero
}

// One returns the multiplicative identity of F.
func One[F FeltLike]() F {
	return F(fp.One())
}

// sharedZero and sharedOne back ZeroPtr and OnePtr; nothing may write through those pointers.
var (
	sharedZero fp.Element
	sharedOne  = fp.One()
)

// ZeroPtr returns the shared zero viewed as F without allocating; callers must not mutate it.
// For a pointer you can write through, use new(F).
func ZeroPtr[F FeltLike]() *F {
	return (*F)(unsafe.Pointer(&sharedZero))
}

// OnePtr returns the shared one viewed as F without allocating; callers must not mutate it.
// For a pointer you can write through, use new(One[F]()).
func OnePtr[F FeltLike]() *F {
	return (*F)(unsafe.Pointer(&sharedOne))
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

// NewRandom creates a new random Felt based type. It also forces a heap allocation.
// For efficient code use `Random`
func NewRandom[F FeltLike]() *F {
	f := Random[F]()
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

// Creates a new random Felt based type
func Random[F FeltLike]() F {
	f := new(Felt).SetRandom()
	return F(*f)
}
