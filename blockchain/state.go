package blockchain

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

type StateProvider interface {
	HeadState() (StateReader, StateCloser, error)
	StateAtBlockHash(blockHash *felt.Felt) (StateReader, StateCloser, error)
	StateAtBlockNumber(blockNumber uint64) (StateReader, StateCloser, error)
}

type StateReader interface {
	ContractReader
	ClassReader
	TrieProvider
}

type StateCloser func() error

type ContractReader interface {
	ContractClassHash(addr *felt.Felt) (*felt.Felt, error)
	ContractNonce(addr *felt.Felt) (*felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (*felt.Felt, error)
}

type ClassReader interface {
	Class(classHash *felt.Felt) (*core.DeclaredClass, error)
}

type TrieProvider interface {
	ClassTrie() (*trie.Trie, error)
	ContractTrie() (*trie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error)
}

// HeadState returns a StateReader that provides a stable view to the latest state
func (b *Blockchain) HeadState() (StateReader, StateCloser, error) {
	b.listener.OnRead("HeadState")
	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, txn.Discard, err
	}

	_, err = ChainHeight(txn)
	if err != nil {
		return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
	}

	st, err := state.New(txn)
	if err != nil {
		return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return st, txn.Discard, nil
}

// StateAtBlockNumber returns a StateReader that provides a stable view to the state at the given block number
func (b *Blockchain) StateAtBlockNumber(blockNumber uint64) (StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockNumber")
	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, txn.Discard, err
	}

	_, err = blockHeaderByNumber(txn, blockNumber)
	if err != nil {
		return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
	}

	st, err := state.NewStateHistory(txn, blockNumber)
	if err != nil {
		return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return st, txn.Discard, nil
}

// StateAtBlockHash returns a StateReader that provides a stable view to the state at the given block hash
func (b *Blockchain) StateAtBlockHash(blockHash *felt.Felt) (StateReader, StateCloser, error) {
	b.listener.OnRead("StateAtBlockHash")
	if blockHash.IsZero() {
		txn := db.NewMemTransaction()
		emptyState, err := state.New(txn)
		if err != nil {
			return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
		}
		return emptyState, txn.Discard, nil
	}

	txn, err := b.database.NewTransaction(false)
	if err != nil {
		return nil, txn.Discard, err
	}

	header, err := blockHeaderByHash(txn, blockHash)
	if err != nil {
		return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
	}

	st, err := state.NewStateHistory(txn, header.Number)
	if err != nil {
		return nil, txn.Discard, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return st, txn.Discard, nil
}
