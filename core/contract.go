package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

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

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset uint
}

// Class unambiguously defines a [Contract]'s semantics.
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
	// The starknet_keccak hash of the ".json" file compiler output.
	ProgramHash *felt.Felt
	Bytecode    []*felt.Felt
}

// Hash computes the [Pedersen Hash] of the class.
//
// [Pedersen Hash]: https://docs.starknet.io/documentation/develop/Contracts/contract-hash/#how_the_class_hash_is_computed
func (c *Class) Hash() (*felt.Felt, error) {
	// TODO
	// zeroNonce := felt.NewFelt(0)
	// apiVersion := zeroNonce.SetUint64(uint64(c.APIVersion))

	// // Pederesen hash of nonce 0 and API version
	// hashState, err := Pedersen(&zeroNonce, apiVersion)
	// if err != nil {
	// 	return nil, err
	// }

	// // Pedersen hash of hashState and Externals
	// hashState, err =
	return nil, nil
}

// Hash computes the address of a StarkNet contract.
func (c *Class) Address() (*felt.Felt, error) {
	// TODO
	return nil, nil
}
