package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

// Class unambiguously defines a [Contract]'s semantics.
type Class struct {
	Abi any
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// An ascii-encoded array of builtin names imported by the class.
	Builtins []*felt.Felt
	// The starknet_keccak hash of the ".json" file compiler output.
	ProgramHash *felt.Felt
	Bytecode    []*felt.Felt
}

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt
}

func (c *Class) Hash() *felt.Felt {
	return crypto.PedersenArray(
		&felt.Zero,
		crypto.PedersenArray(flatten(c.Externals)...),
		crypto.PedersenArray(flatten(c.L1Handlers)...),
		crypto.PedersenArray(flatten(c.Constructors)...),
		crypto.PedersenArray(c.Builtins...),
		c.ProgramHash,
		crypto.PedersenArray(c.Bytecode...),
	)
}

func flatten(entryPoints []EntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because the order
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = entryPoint.Offset
	}
	return result
}
