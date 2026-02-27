package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core State

// State is the mutable Starknet state, extending StateReader with write operations
// to apply/revert block updates and compute the state commitment.
type State interface {
	StateReader

	// Update applies the given state update at blockNum: registers declared classes,
	// deploys contracts, applies storage/nonce/class-hash diffs to the stateand updates the tries.
	// Verifies state roots before and after the update.
	// todo: change declaredClasses map[felt.Felt]ClassDefinition to map[felt.ClassHash]ClassDefinition
	Update(
		blockNum uint64,
		update *StateUpdate,
		declaredClasses map[felt.Felt]ClassDefinition,
		skipVerifyNewRoot bool,
	) error
	// Revert rolls back the state update for blockNum, restoring the state to update.OldRoot.
	// Requires the current root to match update.NewRoot.
	Revert(blockNum uint64, update *StateUpdate) error
	// Commitment returns the current state root hash, derived from the contracts and classes tries.
	// todo: change return type from felt.Felt to felt.StateRootHash
	Commitment() (felt.Felt, error)
}

//go:generate mockgen -destination=../mocks/mock_state_reader.go -package=mocks github.com/NethermindEth/juno/core StateReader

// StateReader provides read-only access to Starknet state:
// contracts, storage, class definitions, and the underlying tries.
type StateReader interface {
	// ContractClassHash returns the class hash of the contract at addr.
	// todo: change add *feltFelt to *felt.Address
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	// ContractNonce returns the nonce of the contract at addr.
	// todo: change add *feltFelt to *felt.Address
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	// ContractStorage returns the value at key in the storage of the contract at addr.
	// todo: change add *feltFelt to *felt.Address
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	// Class returns the class definition and declaration block number for the given class hash.
	// todo: change classHash *felt.Felt to *felt.ClassHash
	Class(classHash *felt.Felt) (*DeclaredClassDefinition, error)
	// CompiledClassHash returns the CASM class hash for a Sierra class hash (Poseidon-based, V1).
	CompiledClassHash(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	// CompiledClassHashV2 returns the CASM class hash for a Sierra class hash (blake2s-based, V2).
	CompiledClassHashV2(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	// ClassTrie returns the trie of all declared classes, keyed by class hash.
	ClassTrie() (Trie, error)
	// ContractTrie returns the global contracts trie, keyed by contract address.
	ContractTrie() (Trie, error)
	// ContractStorageTrie returns the storage trie for the contract at addr.
	// todo: change addr *felt.Felt to *felt.Address
	ContractStorageTrie(addr *felt.Felt) (Trie, error)
}
