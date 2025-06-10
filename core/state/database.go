package state

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

const (
	ClassTrieHeight           = 251
	ContractTrieHeight        = 251
	ContractStorageTrieHeight = 251
)

type StateDB struct {
	disk       db.KeyValueStore
	triedb     *triedb.Database
	stateCache *stateCache
}

func NewStateDB(disk db.KeyValueStore, triedb *triedb.Database) *StateDB {
	return &StateDB{disk: disk, triedb: triedb, stateCache: newStateCache()}
}

// Opens a class trie for the given state root
func (s *StateDB) ClassTrie(stateComm felt.Felt) (*trie2.Trie, error) {
	switch scheme := s.triedb.Scheme(); scheme {
	case triedb.PathDB:
		return trie2.New(trieutils.NewClassTrieID(stateComm), ClassTrieHeight, crypto.Poseidon, s.triedb)
	case triedb.HashDB:
		if stateComm.IsZero() {
			return trie2.New(trieutils.NewClassTrieID(stateComm), ClassTrieHeight, crypto.Poseidon, s.triedb)
		}
		rootHash, err := core.GetClassAndContractRootByStateCommitment(s.disk, &stateComm)
		if err != nil {
			return nil, err
		}
		classRootHash, _, err := trienode.DecodeTriesRoots(rootHash)
		if err != nil {
			return nil, err
		}
		return trie2.NewFromRootHash(trieutils.NewClassTrieID(stateComm), ClassTrieHeight, crypto.Poseidon, s.triedb, &classRootHash)
	default:
		return nil, fmt.Errorf("unsupported trie db type: %T", scheme)
	}
}

// Opens a contract trie for the given state root
func (s *StateDB) ContractTrie(stateComm felt.Felt) (*trie2.Trie, error) {
	switch scheme := s.triedb.Scheme(); scheme {
	case triedb.PathDB:
		return trie2.New(trieutils.NewContractTrieID(stateComm), ContractTrieHeight, crypto.Pedersen, s.triedb)
	case triedb.HashDB:
		if stateComm.IsZero() {
			return trie2.New(trieutils.NewContractTrieID(stateComm), ContractTrieHeight, crypto.Pedersen, s.triedb)
		}
		rootHash, err := core.GetClassAndContractRootByStateCommitment(s.disk, &stateComm)
		if err != nil {
			return nil, err
		}
		contractRootHash, _, err := trienode.DecodeTriesRoots(rootHash)
		if err != nil {
			return nil, err
		}
		return trie2.NewFromRootHash(trieutils.NewContractTrieID(stateComm), ContractTrieHeight, crypto.Pedersen, s.triedb, &contractRootHash)
	default:
		return nil, fmt.Errorf("unsupported trie db type: %T", scheme)
	}
}

// Opens a contract storage trie for the given state root and contract address
func (s *StateDB) ContractStorageTrie(stateComm, owner felt.Felt) (*trie2.Trie, error) {
	switch scheme := s.triedb.Scheme(); scheme {
	case triedb.PathDB:
		return trie2.New(trieutils.NewContractStorageTrieID(stateComm, owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb)
	case triedb.HashDB:
		if stateComm.IsZero() {
			return trie2.New(trieutils.NewContractStorageTrieID(stateComm, owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb)
		}
		rootHash, err := core.GetClassAndContractRootByStateCommitment(s.disk, &stateComm)
		if err != nil {
			return nil, err
		}
		contractRootHash, _, err := trienode.DecodeTriesRoots(rootHash)
		if err != nil {
			return nil, err
		}
		return trie2.NewFromRootHash(trieutils.NewContractStorageTrieID(stateComm, owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb, &contractRootHash)
	default:
		return nil, fmt.Errorf("unsupported trie db type: %T", scheme)
	}
}
