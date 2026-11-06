package slicedata

import "unsafe"

func Data[F ~[4]uint64](out []F) *F { return unsafe.SliceData(out) }
