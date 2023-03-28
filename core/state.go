package core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/bits-and-blooms/bitset"
)

const stateTrieHeight = 251

var _ StateReader = (*State)(nil)

type StateReader interface {
	HeadStateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	// todo: extend
}

type HeadStateReader interface {
	ContractClassHash(addr *felt.Felt) (*felt.Felt, error)
	ContractNonce(addr *felt.Felt) (*felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (*felt.Felt, error)
}

type State struct {
	*History
	txn db.Transaction
}

func NewState(txn db.Transaction) *State {
	return &State{
		History: NewHistory(txn),
		txn:     txn,
	}
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
func (s *State) putNewContract(addr, classHash *felt.Felt) error {
	contract, err := DeployContract(addr, classHash, s.txn)
	if err != nil {
		return err
	}
	return s.updateContractCommitment(contract)
}

// ContractClassHash returns class hash of a contract at a given address.
func (s *State) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}
	return contract.ClassHash()
}

// ContractNonce returns nonce of a contract at a given address.
func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}
	return contract.Nonce()
}

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *State) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	return contract.Storage(key)
}

// Root returns the state commitment.
func (s *State) Root() (*felt.Felt, error) {
	storage, err := s.storage()
	if err != nil {
		return nil, err
	}
	return storage.Root()
}

// storage returns a [core.Trie] that represents the Starknet global state in the given Txn context.
func (s *State) storage() (*trie.Trie, error) {
	tTxn := NewTransactionStorage(s.txn, []byte{byte(db.StateTrie)})

	rootKey, err := s.rootKey()
	if err != nil {
		if !errors.Is(db.ErrKeyNotFound, err) {
			return nil, err
		}
		rootKey = nil
	}

	return trie.NewTriePedersen(tTxn, stateTrieHeight, rootKey), nil
}

// rootKey returns key to the root node in the given Txn context.
func (s *State) rootKey() (*bitset.BitSet, error) {
	var key *bitset.BitSet
	if err := s.txn.Get(db.StateRootKey.Key(), func(val []byte) error {
		key = new(bitset.BitSet)
		return key.UnmarshalBinary(val)
	}); err != nil {
		return nil, err
	}
	return key, nil
}

// updateStateRootDBKey updates the fields related to the state trie root in
// the given Txn context.
func (s *State) updateStateRootDBKey(state *trie.Trie) error {
	rootKeyDBKey := db.StateRootKey.Key()
	if rootKey := state.RootKey(); rootKey != nil {
		rootKeyBytes, err := rootKey.MarshalBinary()
		if err != nil {
			return err
		}

		if err := s.txn.Set(rootKeyDBKey, rootKeyBytes); err != nil {
			return err
		}
	} else if err := s.txn.Delete(rootKeyDBKey); err != nil {
		return err
	}

	return nil
}

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(blockNumber uint64, update *StateUpdate, declaredClasses map[felt.Felt]Class) error {
	currentRoot, err := s.Root()
	if err != nil {
		return err
	}
	if !update.OldRoot.Equal(currentRoot) {
		return fmt.Errorf("state's current root: %s does not match state update's old root: %s", currentRoot, update.OldRoot)
	}

	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	// Todo: add test cases for retrieving Classes when State struct is extended to return Classes.
	for classHash, class := range declaredClasses {
		if err = s.putClass(&classHash, class); err != nil {
			return err
		}
	}

	// register deployed contracts
	for _, contract := range update.StateDiff.DeployedContracts {
		if err = s.putNewContract(contract.Address, contract.ClassHash); err != nil {
			return err
		}
	}

	// update contract nonces
	for addr, nonce := range update.StateDiff.Nonces {
		if err = s.updateContractNonce(&addr, nonce); err != nil {
			return err
		}
	}

	// update contract storages
	for addr, diff := range update.StateDiff.StorageDiffs {
		if err != nil {
			return err
		}
		if err = s.updateContractStorage(&addr, diff, blockNumber); err != nil {
			return err
		}
	}

	newRoot, err := s.Root()
	if err != nil {
		return err
	}
	if !update.NewRoot.Equal(newRoot) {
		return fmt.Errorf("state's new root: %s does not match state update's new root: %s", newRoot, update.NewRoot)
	}
	return nil
}

func (s *State) putClass(classHash *felt.Felt, class Class) error {
	classKey := db.Class.Key(classHash.Marshal())

	err := s.txn.Get(classKey, func(val []byte) error {
		return nil
	})

	if errors.Is(err, db.ErrKeyNotFound) {
		classEncoded, encErr := encoder.Marshal(class)
		if encErr != nil {
			return encErr
		}

		return s.txn.Set(classKey, classEncoded)
	}
	return err
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorage(addr *felt.Felt, diff []StorageDiff, blockNumber uint64) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	onValueChanged := func(location, oldValue *felt.Felt) error {
		return s.LogContractStorage(addr, location, oldValue, blockNumber)
	}

	if err = contract.UpdateStorage(diff, onValueChanged); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(addr, nonce *felt.Felt) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	if err := contract.UpdateNonce(nonce); err != nil {
		return err
	}

	return s.updateContractCommitment(contract)
}

// updateContractCommitment recalculates the contract commitment and updates its value in the global state Trie
func (s *State) updateContractCommitment(contract *Contract) error {
	root, err := contract.Root()
	if err != nil {
		return err
	}

	cHash, err := contract.ClassHash()
	if err != nil {
		return err
	}

	nonce, err := contract.Nonce()
	if err != nil {
		return err
	}

	commitment := calculateContractCommitment(root, cHash, nonce)

	state, err := s.storage()
	if err != nil {
		return err
	}

	if _, err = state.Put(contract.Address, commitment); err != nil {
		return err
	}

	return s.updateStateRootDBKey(state)
}

func calculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(classHash, storageRoot), nonce), &felt.Zero)
}
