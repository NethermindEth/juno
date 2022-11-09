package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

// Class unambiguously defines a Contract's semantics.
type Class struct {
	// The version of the class, currently always 0.
	APIVersion uint	
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// An ascii-encoded array of builtin names imported by the class.
	Builtins []string
	// The starknet_keccak hash of the JSON compiler output.
	ProgramHash []byte
	Bytecode []*felt.Felt
}

func (c *Class) Hash() {
	// TODO: see
	// https://docs.starknet.io/documentation/develop/Contracts/contract-hash/#how_the_class_hash_is_computed
}
