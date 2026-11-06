package plainunsafe

import "unsafe"

func Felt[F ~[4]uint64](dst *F) *[4]uint64  { return (*[4]uint64)(unsafe.Pointer(dst)) }
func Offset(base *[4]uint64, i int) *uint64 { return (*uint64)(unsafe.Add(unsafe.Pointer(base), i*8)) }
func Slice(dst *[4]uint64) []uint64         { return unsafe.Slice((*uint64)(unsafe.Pointer(dst)), 4) }
func Width() (uintptr, uintptr)             { return unsafe.Sizeof([4]uint64{}), unsafe.Alignof([4]uint64{}) }
