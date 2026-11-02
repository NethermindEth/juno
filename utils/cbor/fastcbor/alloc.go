package fastcbor

import (
	"reflect"
	"unsafe"
)

// The plans allocate values of types known only at run time. reflect.New and growing a slice
// through reflect do that, but first look up the type of a pointer to what they allocate, in a
// sync.Map for the types the program never names, which cost 4 to 9% of a receipt or a header.
// These are the runtime's allocators behind reflect, which take the type itself. The runtime keeps
// them for the packages that link to them (go.dev/issue/67401).

// runtimeType is the runtime's description of a type, which a reflect.Type points to.
type runtimeType struct{}

// runtimeTypeOf returns the runtime's description of t: the data word of the interface.
func runtimeTypeOf(t reflect.Type) *runtimeType {
	return (*runtimeType)((*[2]unsafe.Pointer)(unsafe.Pointer(&t))[1])
}

// unsafe_New returns a new zeroed value of the type.
//
//go:linkname unsafe_New reflect.unsafe_New
//nolint:revive // the name is the runtime's
func unsafe_New(*runtimeType) unsafe.Pointer

// unsafe_NewArray returns a new zeroed array of n values of the type.
//
//go:linkname unsafe_NewArray reflect.unsafe_NewArray
//nolint:revive // the name is the runtime's
func unsafe_NewArray(typ *runtimeType, n int) unsafe.Pointer

// makemap returns a new map of the type with room for cap entries.
//
//go:linkname makemap reflect.makemap
func makemap(t *runtimeType, cap int) unsafe.Pointer

// mapassign0 stores a copy of the value val points to under a copy of the key key points to.
//
//go:linkname mapassign0 reflect.mapassign0
//nolint:revive // the name is the runtime's
func mapassign0(t *runtimeType, m, key, val unsafe.Pointer)

// typedmemclr zeroes the value of the type that ptr points to.
//
//go:linkname typedmemclr reflect.typedmemclr
func typedmemclr(t *runtimeType, ptr unsafe.Pointer)

// mapassign returns where the map keeps the value of the key, which is zeroed if the map had no
// such key. The value stays valid until the map next grows.
//
//go:linkname mapassign runtime.mapassign
func mapassign(t *runtimeType, m, key unsafe.Pointer) unsafe.Pointer

// mapassign_fast32 is mapassign for a map whose keys are 4 bytes of integer.
//
//go:linkname mapassign_fast32 runtime.mapassign_fast32
//nolint:revive // the name is the runtime's
func mapassign_fast32(t *runtimeType, m unsafe.Pointer, key uint32) unsafe.Pointer

// mapassign_fast64 is mapassign for a map whose keys are 8 bytes of integer.
//
//go:linkname mapassign_fast64 runtime.mapassign_fast64
//nolint:revive // the name is the runtime's
func mapassign_fast64(t *runtimeType, m unsafe.Pointer, key uint64) unsafe.Pointer
