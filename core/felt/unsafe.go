package felt

import (
	"unsafe"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// CastPointerSlice reinterprets s as pointers to another felt type, sharing its backing array.
func CastPointerSlice[To, From FeltLike](s []*From) []*To {
	return unsafe.Slice((**To)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
}

// CastSlice reinterprets s as another felt type, sharing its backing array.
func CastSlice[To, From FeltLike](s []From) []To {
	return unsafe.Slice((*To)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
}

// ImplOf returns the field element behind any felt type without copying it.
func ImplOf[F FeltLike](f *F) *fp.Element {
	return (*fp.Element)(unsafe.Pointer(f))
}
