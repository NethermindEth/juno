package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core State
type State interface {
	StateReader

	Update(
		blockNum uint64,
		update *StateUpdate,
		declaredClasses map[felt.Felt]ClassDefinition,
		skipVerifyNewRoot bool,
		flushChanges bool,
	) error
	Revert(blockNum uint64, update *StateUpdate) error
	Commitment() (felt.Felt, error)
}

//go:generate mockgen -destination=../mocks/mock_state_reader.go -package=mocks github.com/NethermindEth/juno/core StateReader
type StateReader interface {
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClassDefinition, error)
	CompiledClassHash(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	CompiledClassHashV2(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	ClassTrie() (Trie, error)
	ContractTrie() (Trie, error)
	ContractStorageTrie(addr *felt.Felt) (Trie, error)
}
