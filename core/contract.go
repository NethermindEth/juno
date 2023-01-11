package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

// Class unambiguously defines a [Contract]'s semantics.
type Class struct {
	// The version of the class, currently always 0.
	APIVersion *felt.Felt
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

func (c *Class) Hash() (*felt.Felt, error) {
	externalsHash, err := crypto.PedersenArray(flatten(c.Externals)...)
	if err != nil {
		return nil, err
	}
	l1HandlersHash, err := crypto.PedersenArray(flatten(c.L1Handlers)...)
	if err != nil {
		return nil, err
	}
	constructorsHash, err := crypto.PedersenArray(flatten(c.Constructors)...)
	if err != nil {
		return nil, err
	}
	builtinsHash, err := crypto.PedersenArray(c.Builtins...)
	if err != nil {
		return nil, err
	}
	bytecodeHash, err := crypto.PedersenArray(c.Bytecode...)
	if err != nil {
		return nil, err
	}
	return crypto.PedersenArray(
		c.APIVersion,
		externalsHash,
		l1HandlersHash,
		constructorsHash,
		builtinsHash,
		c.ProgramHash,
		bytecodeHash,
	)
}

func flatten(entryPoints []EntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
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
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt
}

// Contract is an instance of a [Class].
type Contract struct {
	// The number of transactions sent from this contract.
	// Only account contracts can have a non-zero nonce.
	Nonce uint
	// Hash of the class that this contract instantiates.
	ClassHash *felt.Felt
	// Root of the contract's storage trie.
	StorageRoot *felt.Felt // TODO: is this field necessary?
}
