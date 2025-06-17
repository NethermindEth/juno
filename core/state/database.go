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
	stateCache := newStateCache()
	return &StateDB{disk: disk, triedb: triedb, stateCache: &stateCache}
}

type trieConfig struct {
	id      func(felt.Felt) trieutils.TrieID
	height  uint8
	getRoot func([]byte) (*felt.Felt, error)
}

func (s *StateDB) createTrie(stateComm *felt.Felt, config trieConfig) (*trie2.Trie, error) {
	switch scheme := s.triedb.Scheme(); scheme {
	case triedb.PathDB:
		return trie2.New(config.id(*stateComm), config.height, crypto.Poseidon, s.triedb)
	case triedb.HashDB:
		if stateComm.IsZero() {
			return trie2.New(config.id(*stateComm), config.height, crypto.Poseidon, s.triedb)
		}
		rootsBytes, err := core.GetClassAndContractRootByStateCommitment(s.disk, stateComm)
		if err != nil {
			return nil, err
		}
		rootHash, err := config.getRoot(rootsBytes)
		if err != nil {
			return nil, err
		}
		return trie2.NewFromRootHash(config.id(*stateComm), config.height, crypto.Poseidon, s.triedb, rootHash)
	default:
		return nil, fmt.Errorf("unsupported trie db type: %T", scheme)
	}
}

func (s *StateDB) ClassTrie(stateComm *felt.Felt) (*trie2.Trie, error) {
	return s.createTrie(stateComm, trieConfig{
		id:     func(f felt.Felt) trieutils.TrieID { return trieutils.NewClassTrieID(f) },
		height: ClassTrieHeight,
		getRoot: func(rootsBytes []byte) (*felt.Felt, error) {
			classRoot, _, err := trienode.DecodeTriesRoots(rootsBytes)
			if err != nil {
				return nil, err
			}
			return &classRoot, nil
		},
	})
}

func (s *StateDB) ContractTrie(stateComm *felt.Felt) (*trie2.Trie, error) {
	return s.createTrie(stateComm, trieConfig{
		id:     func(f felt.Felt) trieutils.TrieID { return trieutils.NewContractTrieID(f) },
		height: ContractTrieHeight,
		getRoot: func(rootsBytes []byte) (*felt.Felt, error) {
			_, contractRoot, err := trienode.DecodeTriesRoots(rootsBytes)
			if err != nil {
				return nil, err
			}
			return &contractRoot, nil
		},
	})
}

// Opens a contract storage trie for the given state root and contract address
func (s *StateDB) ContractStorageTrie(stateComm, owner *felt.Felt) (*trie2.Trie, error) {
	switch scheme := s.triedb.Scheme(); scheme {
	case triedb.PathDB:
		return trie2.New(trieutils.NewContractStorageTrieID(*stateComm, *owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb)
	case triedb.HashDB:
		if stateComm.IsZero() {
			return trie2.New(trieutils.NewContractStorageTrieID(*stateComm, *owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb)
		}
		rootsBytes, err := core.GetClassAndContractRootByStateCommitment(s.disk, stateComm)
		if err != nil {
			return nil, err
		}
		_, contractRootHash, err := trienode.DecodeTriesRoots(rootsBytes)
		if err != nil {
			return nil, err
		}
		contractTrie, err := trie2.NewFromRootHash(
			trieutils.NewContractTrieID(*stateComm),
			ContractTrieHeight,
			crypto.Pedersen,
			s.triedb,
			&contractRootHash,
		)
		if err != nil {
			return nil, err
		}

		contractComm, err := contractTrie.Get(owner)
		if err != nil {
			return nil, err
		}

		if contractComm.IsZero() {
			return trie2.New(trieutils.NewContractStorageTrieID(*stateComm, *owner), ContractStorageTrieHeight, crypto.Pedersen, s.triedb)
		}

		contractStorageRootBytes, err := core.GetContractStorageRoot(s.disk, stateComm, &contractComm)
		if err != nil {
			return nil, err
		}
		contractStorageRoot := new(felt.Felt).SetBytes(contractStorageRootBytes)

		return trie2.NewFromRootHash(
			trieutils.NewContractStorageTrieID(*stateComm, *owner),
			ContractStorageTrieHeight,
			crypto.Pedersen,
			s.triedb,
			contractStorageRoot,
		)
	default:
		return nil, fmt.Errorf("unsupported trie db type: %T", scheme)
	}
}
