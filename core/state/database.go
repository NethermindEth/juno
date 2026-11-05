package state

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/triedb/database"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

const (
	ClassTrieHeight           = 251
	ContractTrieHeight        = 251
	ContractStorageTrieHeight = 251
)

type StateDB struct {
	disk   db.KeyValueStore
	triedb database.TrieDB
}

func NewStateDB(disk db.KeyValueStore, triedb database.TrieDB) *StateDB {
	return &StateDB{disk: disk, triedb: triedb}
}

// Opens a class trie for the given state root
func (s *StateDB) ClassTrie(stateComm *felt.StateRootHash) (*trie2.Trie, error) {
	return trie2.New(
		trieutils.NewClassTrieID(*stateComm),
		ClassTrieHeight,
		crypto.Poseidon,
		s.triedb,
	)
}

// Opens a contract trie for the given state root
func (s *StateDB) ContractTrie(stateComm *felt.StateRootHash) (*trie2.Trie, error) {
	return trie2.New(
		trieutils.NewContractTrieID(*stateComm),
		ContractTrieHeight,
		crypto.Pedersen,
		s.triedb,
	)
}

// Opens a contract storage trie for the given state root and contract address
func (s *StateDB) ContractStorageTrie(
	stateComm *felt.StateRootHash,
	owner *felt.Address,
) (*trie2.Trie, error) {
	return trie2.New(
		trieutils.NewContractStorageTrieID(
			*stateComm,
			*owner,
		),
		ContractStorageTrieHeight,
		crypto.Pedersen,
		s.triedb,
	)
}
