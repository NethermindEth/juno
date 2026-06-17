//go:build cgo && !no_rust_crypto

package crypto

/*
#cgo vm_debug  LDFLAGS: -L${SRCDIR}/rust/target/debug   -ljuno_crypto_rs
#cgo !vm_debug LDFLAGS: -L${SRCDIR}/rust/target/release -ljuno_crypto_rs
#cgo linux     LDFLAGS: -lm -ldl
#include <stdint.h>
void juno_hades_permutation(uint64_t* state);
*/
import "C"

import (
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
)

// hadesPermutationRounds runs the Hades rounds in place via the Rust
// implementation over cgo. felt.Felt is four little-endian Montgomery limbs, so
// a [3]felt.Felt is 12 contiguous u64 passed by pointer with no conversion or
// allocation. Output is bit-identical to the pure-Go fallback (guarded by tests,
// and verified against the byte-marshalled path over 1M random inputs).
func hadesPermutationRounds(state *[3]felt.Felt) {
	C.juno_hades_permutation((*C.uint64_t)(unsafe.Pointer(&state[0])))
}
