package compiler

import (
	"context"

	"github.com/NethermindEth/juno/starknet"
)

// Compiler compiles Sierra classes to CASM.
type Compiler interface {
	Compile(ctx context.Context, sierra *starknet.SierraClass) (
		*starknet.CasmClass, error,
	)
}

type inProcessCompiler struct{}

// NewUnsafe returns a Compiler that compiles in the same process Juno is running.
// It can be unsafe if the compilation process get stuck.
func NewUnsafe() Compiler {
	return &inProcessCompiler{}
}

// Compile runs Sierra-to-CASM compilation as a process thread
// If there were to be a bug in the compilation process, such as an infinite loop
// it opens a vector for a DoS attack.
func (r *inProcessCompiler) Compile(
	_ context.Context, sierra *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	return CompileFFI(sierra)
}
