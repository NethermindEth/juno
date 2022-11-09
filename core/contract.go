package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

// Contract is an instance of a Class.
type Contract struct {
	// The number of transactions sent from this contract.
	// Only account contracts can have a non-zero nonce.
	Nonce       uint
	// Hash of the class that this contract instantiates.
	ClassHash   *felt.Felt
	// Root of the contract's storage trie.
	StorageRoot *felt.Felt // TODO: is this field necessary?
}
