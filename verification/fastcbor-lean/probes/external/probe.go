package external

import "unsafe"

type runtimeType struct{}

//go:linkname unsafe_New reflect.unsafe_New
func unsafe_New(*runtimeType) unsafe.Pointer
