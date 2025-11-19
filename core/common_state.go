package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

//go:generate mockgen -destination=../mocks/mock_common_state.go -package=mocks github.com/NethermindEth/juno/core CommonState
type CommonState interface {
	CommonStateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)

	Update(
		blockNum uint64,
		update *StateUpdate,
		declaredClasses map[felt.Felt]ClassDefinition,
		skipVerifyNewRoot bool,
	) error
	Revert(blockNum uint64, update *StateUpdate) error
	Commitment() (felt.Felt, error)
}

type CommonStateReader interface {
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClassDefinition, error)

	ClassTrie() (CommonTrie, error)
	ContractTrie() (CommonTrie, error)
	ContractStorageTrie(addr *felt.Felt) (CommonTrie, error)
	CompiledClassHash(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	CompiledClassHashV2(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
}
