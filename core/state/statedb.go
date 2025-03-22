package state

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

const (
	ClassTrieHeight    = 251
	ContractTrieHeight = 251
)

type StateDB struct {
	disk   db.KeyValueStore
	triedb *triedb.Database
	// TODO: add cache related stuff here
}

func NewStateDB(disk db.KeyValueStore, triedb *triedb.Database) *StateDB {
	return &StateDB{disk: disk, triedb: triedb}
}

// Opens a class trie for the given state root
func (s *StateDB) ClassTrie(stateRoot felt.Felt) (*trie2.Trie, error) {
	return trie2.New(trieutils.NewClassTrieID(stateRoot), ClassTrieHeight, crypto.Poseidon, s.triedb)
}

// Opens a contract trie for the given state root
func (s *StateDB) ContractTrie(stateRoot felt.Felt) (*trie2.Trie, error) {
	return trie2.New(trieutils.NewContractTrieID(stateRoot), ContractTrieHeight, crypto.Pedersen, s.triedb)
}

// Opens a contract storage trie for the given state root and contract address
func (s *StateDB) ContractStorageTrie(stateRoot, owner felt.Felt) (*trie2.Trie, error) {
	return trie2.New(trieutils.NewContractStorageTrieID(stateRoot, owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb)
}
