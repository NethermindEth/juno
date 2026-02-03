package types

import (
	"crypto/rand"

	"github.com/holiman/uint256"
)

type U256Like interface {
	~[4]uint64
}

func NewFromUint64[T U256Like](num uint64) *T {
	v := FromUint64[T](num)
	return &v
}

func NewFromString[T U256Like](val string) (*T, error) {
	v, err := FromString[T](val)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func NewUnsafeFromString[T U256Like](val string) *T {
	v := UnsafeFromString[T](val)
	return &v
}

func NewFromBytes[T U256Like](val []byte) *T {
	v := FromBytes[T](val)
	return &v
}

func FromUint64[T U256Like](num uint64) T {
	u := uint256.NewInt(num)
	return T(*u)
}

func FromBytes[T U256Like](value []byte) T {
	u := new(U256)
	if err := u.SetBytesCanonical(value); err != nil {
		panic(err)
	}
	return T(*u)
}

func FromString[T U256Like](value string) (T, error) {
	u, err := u256FromString(value)
	if err != nil {
		var zero T
		return zero, err
	}
	return T(*u), nil
}

func UnsafeFromString[T U256Like](value string) T {
	u, err := u256FromString(value)
	if err != nil {
		panic(err)
	}
	return T(*u)
}

// Creates a new random U256 based type
func Random[T U256Like]() T {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	u := new(U256)
	(*uint256.Int)(u).SetBytes(b[:])
	return T(*u)
}
