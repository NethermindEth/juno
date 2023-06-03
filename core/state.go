package core

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

const globalTrieHeight = 251

var (
	stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))
	leafVersion  = new(felt.Felt).SetBytes([]byte(`CONTRACT_CLASS_LEAF_V0`))
)

var _ StateHistoryReader = (*State)(nil)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core StateHistoryReader
type StateHistoryReader interface {
	StateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error)
	ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (*felt.Felt, error)
	ContractNonce(addr *felt.Felt) (*felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (*felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClass, error)
}

type State struct {
	*History
	txn db.Transaction

	stateTrie   *trie.Trie
	classesTrie *trie.Trie
}

func NewState(txn db.Transaction) (*State, error) {
	makeTrie := func(bucket db.Bucket, newTrie trie.NewTrieFunc) (*trie.Trie, error) {
		dbPrefix := bucket.Key()
		tTxn := trie.NewTransactionStorage(txn, dbPrefix)

		gTrie, err := newTrie(tTxn, globalTrieHeight)
		if err != nil {
			return nil, err
		}

		return gTrie, nil
	}

	stateTrie, err := makeTrie(db.StateTrie, trie.NewTriePedersen)
	if err != nil {
		return nil, err
	}

	classesTrie, err := makeTrie(db.ClassesTrie, trie.NewTriePoseidon)
	if err != nil {
		return nil, err
	}

	return &State{
		History:     NewHistory(txn),
		txn:         txn,
		stateTrie:   stateTrie,
		classesTrie: classesTrie,
	}, nil
}

// putNewContract creates a contract storage instance in the state and stores the relation between contract address and class hash to be
// queried later with [GetContractClass].
func (s *State) putNewContract(stateTrie *trie.Trie, addr, classHash *felt.Felt, blockNumber uint64) error {
	contract, err := DeployContract(addr, classHash, s.txn)
	if err != nil {
		return err
	}

	var numBytes [8]byte
	binary.BigEndian.PutUint64(numBytes[:], blockNumber)
	if err = s.txn.Set(db.ContractDeploymentHeight.Key(addr.Marshal()), numBytes[:]); err != nil {
		return err
	}

	return s.updateContractCommitment(stateTrie, contract)
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
	var storageRoot, classesRoot *felt.Felt
	var err error

	if storageRoot, err = s.stateTrie.Root(); err != nil {
		return nil, err
	}

	if classesRoot, err = s.classesTrie.Root(); err != nil {
		return nil, err
	}

	if classesRoot.IsZero() {
		return storageRoot, nil
	}

	return crypto.PoseidonArray(stateVersion, storageRoot, classesRoot), nil
}

// Update applies a StateUpdate to the State object. State is not
// updated if an error is encountered during the operation. If update's
// old or new root does not match the state's old or new roots,
// [ErrMismatchedRoot] is returned.
func (s *State) Update(blockNumber uint64, diff *StateDiff, declaredClasses map[felt.Felt]Class) error {
	var err error

	// register declared classes mentioned in stateDiff.deployedContracts and stateDiff.declaredClasses
	// Todo: add test cases for retrieving Classes when State struct is extended to return Classes.
	for classHash, class := range declaredClasses {
		if err = s.putClass(&classHash, class, blockNumber); err != nil {
			return err
		}
	}

	if err = s.updateDeclaredClasses(diff.DeclaredV1Classes); err != nil {
		return err
	}

	if err = s.updateContracts(blockNumber, diff); err != nil {
		return err
	}

	return nil
}

func (s *State) updateContracts(blockNumber uint64, diff *StateDiff) error {
	// register deployed contracts
	for _, contract := range diff.DeployedContracts {
		if err := s.putNewContract(s.stateTrie, contract.Address, contract.ClassHash, blockNumber); err != nil {
			return err
		}
	}

	// replace contract instances
	for _, replace := range diff.ReplacedClasses {
		oldClassHash, err := s.replaceContract(s.stateTrie, replace.Address, replace.ClassHash)
		if err != nil {
			return err
		}

		if err = s.LogContractClassHash(replace.Address, oldClassHash, blockNumber); err != nil {
			return err
		}
	}

	// update contract nonces
	for addr, nonce := range diff.Nonces {
		oldNonce, err := s.updateContractNonce(s.stateTrie, &addr, nonce)
		if err != nil {
			return err
		}

		if err = s.LogContractNonce(&addr, oldNonce, blockNumber); err != nil {
			return err
		}
	}

	// update contract storages
	for addr, storageDiff := range diff.StorageDiffs {
		onValueChanged := func(location, oldValue *felt.Felt) error {
			return s.LogContractStorage(&addr, location, oldValue, blockNumber)
		}

		if err := s.updateContractStorage(s.stateTrie, &addr, storageDiff, onValueChanged); err != nil {
			return err
		}
	}

	return s.stateTrie.Commit()
}

// replaceContract replaces the class that a contract at a given address instantiates
func (s *State) replaceContract(stateTrie *trie.Trie, addr, classHash *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldClassHash, err := contract.ClassHash()
	if err != nil {
		return nil, err
	}

	if err = contract.Replace(classHash); err != nil {
		return nil, err
	}

	if err = s.updateContractCommitment(stateTrie, contract); err != nil {
		return nil, err
	}

	return oldClassHash, nil
}

type DeclaredClass struct {
	At    uint64
	Class Class
}

func (s *State) putClass(classHash *felt.Felt, class Class, declaredAt uint64) error {
	classKey := db.Class.Key(classHash.Marshal())

	err := s.txn.Get(classKey, func(val []byte) error {
		return nil
	})

	if errors.Is(err, db.ErrKeyNotFound) {
		classEncoded, encErr := encoder.Marshal(DeclaredClass{
			At:    declaredAt,
			Class: class,
		})
		if encErr != nil {
			return encErr
		}

		return s.txn.Set(classKey, classEncoded)
	}
	return err
}

// Class returns the class object corresponding to the given classHash
func (s *State) Class(classHash *felt.Felt) (*DeclaredClass, error) {
	classKey := db.Class.Key(classHash.Marshal())

	var class DeclaredClass
	err := s.txn.Get(classKey, func(val []byte) error {
		return encoder.Unmarshal(val, &class)
	})
	if err != nil {
		return nil, err
	}
	return &class, nil
}

// updateContractStorage applies the diff set to the Trie of the
// contract at the given address in the given Txn context.
func (s *State) updateContractStorage(stateTrie *trie.Trie, addr *felt.Felt, diff []StorageDiff, onChanged OnValueChanged) error {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return err
	}

	if err = contract.UpdateStorage(diff, onChanged); err != nil {
		return err
	}

	return s.updateContractCommitment(stateTrie, contract)
}

// updateContractNonce updates nonce of the contract at the
// given address in the given Txn context.
func (s *State) updateContractNonce(stateTrie *trie.Trie, addr, nonce *felt.Felt) (*felt.Felt, error) {
	contract, err := NewContract(addr, s.txn)
	if err != nil {
		return nil, err
	}

	oldNonce, err := contract.Nonce()
	if err != nil {
		return nil, err
	}

	if err = contract.UpdateNonce(nonce); err != nil {
		return nil, err
	}

	if err = s.updateContractCommitment(stateTrie, contract); err != nil {
		return nil, err
	}

	return oldNonce, nil
}

// updateContractCommitment recalculates the contract commitment and updates its value in the global state Trie
func (s *State) updateContractCommitment(stateTrie *trie.Trie, contract *Contract) error {
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

	_, err = stateTrie.Put(contract.Address, commitment)
	return err
}

func calculateContractCommitment(storageRoot, classHash, nonce *felt.Felt) *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(classHash, storageRoot), nonce), &felt.Zero)
}

func (s *State) updateDeclaredClasses(declaredClasses []DeclaredV1Class) error {
	var err error
	for _, declaredClass := range declaredClasses {
		// https://docs.starknet.io/documentation/starknet_versions/upcoming_versions/#commitment
		leafValue := crypto.Poseidon(leafVersion, declaredClass.CompiledClassHash)
		if _, err = s.classesTrie.Put(declaredClass.ClassHash, leafValue); err != nil {
			return err
		}
	}

	return s.classesTrie.Commit()
}

// ContractIsAlreadyDeployedAt returns if contract at given addr was deployed at blockNumber
func (s *State) ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	var deployedAt uint64
	if err := s.txn.Get(db.ContractDeploymentHeight.Key(addr.Marshal()), func(bytes []byte) error {
		deployedAt = binary.BigEndian.Uint64(bytes)
		return nil
	}); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return deployedAt <= blockNumber, nil
}
