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

func GetContractCommitment(storageRoot, classHash *felt.Felt) (*felt.Felt, error) {
	commitment, err := crypto.Pedersen(classHash, storageRoot)
	if err != nil {
		return nil, err
	}
	commitment, err = crypto.Pedersen(commitment, new(felt.Felt))
	if err != nil {
		return nil, err
	}
	return crypto.Pedersen(commitment, new(felt.Felt))
}

// putNewContractWithTxn creates a contract storage instance in the state and stores the relation
// between contract address and class hash to be queried later on with [GetContractClass]
func (s *State) putNewContract(addr, classHash *felt.Felt, txn *badger.Txn) error {
	key := db.Key(db.ContractClassHash, addr.Marshal())
	if _, err := txn.Get(key); err == nil {
		return errors.New("existing contract")
	} else if err = txn.Set(key, classHash.Marshal()); err != nil {
		return err
	} else if _, err := GetContractCommitment(new(felt.Felt), classHash); err != nil { // todo: write commitment to state trie
		return err
	}
	return nil
}

// GetContractClass returns class hash of a contract at a given address
func (s *State) GetContractClass(addr *felt.Felt) (*felt.Felt, error) {
	var classHash *felt.Felt

	return classHash, s.db.View(func(txn *badger.Txn) error {
		var err error
		classHash, err = s.getContractClass(addr, txn)
		return err
	})
}

// getContractClass returns class hash of a contract at a given address in the given Txn context
func (s *State) getContractClass(addr *felt.Felt, txn *badger.Txn) (*felt.Felt, error) {
	var classHash *felt.Felt

	key := db.Key(db.ContractClassHash, addr.Marshal())
	if item, err := txn.Get(key); err != nil {
		return classHash, err
	} else {
		return classHash, item.Value(func(val []byte) error {
			classHash = new(felt.Felt).SetBytes(val)
			return nil
		})
	}
}
