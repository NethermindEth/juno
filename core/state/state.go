package state

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/dgraph-io/badger/v3"
)

type State struct {
	db *badger.DB
}

func NewState(db *badger.DB) *State {
	state := &State{
		db: db,
	}
	return state
}

func CalculateContractCommitment(storageRoot, classHash *felt.Felt) *felt.Felt {
	commitment := crypto.Pedersen(classHash, storageRoot)
	commitment = crypto.Pedersen(commitment, new(felt.Felt))
	return crypto.Pedersen(commitment, new(felt.Felt))
}

// putNewContract creates a contract storage instance in the state and
// stores the relation between contract address and class hash to be
// queried later on with [GetContractClass].
func (s *State) putNewContract(addr, classHash *felt.Felt, txn *badger.Txn) error {
	key := db.ContractClassHash.Key(addr.Marshal())
	if _, err := txn.Get(key); err == nil {
		// Should not happen.
		return errors.New("existing contract")
	} else if err = txn.Set(key, classHash.Marshal()); err != nil {
		return err
	} else {
		CalculateContractCommitment(new(felt.Felt), classHash)
		// todo: write commitment to state trie
	}
	return nil
}

// GetContractClass returns class hash of a contract at a given address.
func (s *State) GetContractClass(addr *felt.Felt) (*felt.Felt, error) {
	var classHash *felt.Felt

	return classHash, s.db.View(func(txn *badger.Txn) error {
		var err error
		classHash, err = s.getContractClass(addr, txn)
		return err
	})
}

// getContractClass returns class hash of a contract at a given address
// in the given Txn context.
func (s *State) getContractClass(addr *felt.Felt, txn *badger.Txn) (*felt.Felt, error) {
	var classHash *felt.Felt

	key := db.ContractClassHash.Key(addr.Marshal())
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return classHash, item.Value(func(val []byte) error {
		classHash = new(felt.Felt).SetBytes(val)
		return nil
	})
}
