package felt

import "unsafe"

// asFeltPtr reinterprets a *FeltLike as a *Felt.
// It is safe because every FeltLike has the exact same memory layout.
func asFeltPtr[F FeltLike](val *F) *Felt {
	return (*Felt)(unsafe.Pointer(val))
}
