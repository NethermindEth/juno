package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// Class unambiguously defines a [Contract]'s semantics.
type Class struct {
	// The version of the class, currently always 0.
	APIVersion *fp.Element
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// An ascii-encoded array of builtin names imported by the class.
	Builtins []*fp.Element
	// The starknet_keccak hash of the ".json" file compiler output.
	ProgramHash *fp.Element
	Bytecode    []*fp.Element
}

func (c *Class) Hash() *fp.Element {
	return crypto.PedersenArray(
		c.APIVersion,
		crypto.PedersenArray(flatten(c.Externals)...),
		crypto.PedersenArray(flatten(c.L1Handlers)...),
		crypto.PedersenArray(flatten(c.Constructors)...),
		crypto.PedersenArray(c.Builtins...),
		c.ProgramHash,
		crypto.PedersenArray(c.Bytecode...),
	)
}

func flatten(entryPoints []EntryPoint) []*fp.Element {
	result := make([]*fp.Element, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because it
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = entryPoint.Offset
	}
	return result
}

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *fp.Element
	// The offset of the instruction in the class's bytecode.
	Offset *fp.Element
}

// Contract is an instance of a [Class].
type Contract struct {
	// The number of transactions sent from this contract.
	// Only account contracts can have a non-zero nonce.
	Nonce uint
	// Hash of the class that this contract instantiates.
	ClassHash *fp.Element
	// Root of the contract's storage trie.
	StorageRoot *fp.Element // TODO: is this field necessary?
}
