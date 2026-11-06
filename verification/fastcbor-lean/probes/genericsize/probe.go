package genericsize

import "unsafe"

func Width[T any](dst *T) uintptr     { return unsafe.Sizeof(*dst) }
func Alignment[T any](dst *T) uintptr { return unsafe.Alignof(*dst) }
