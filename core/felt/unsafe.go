package felt

import "unsafe"

// CastPointerSlice reinterprets s as pointers to another felt type, sharing its backing array.
func CastPointerSlice[To, From FeltLike](s []*From) []*To {
	return unsafe.Slice((**To)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
}

// CastSlice reinterprets s as another felt type, sharing its backing array.
func CastSlice[To, From FeltLike](s []From) []To {
	return unsafe.Slice((*To)(unsafe.Pointer(unsafe.SliceData(s))), len(s))
}
