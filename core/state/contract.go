package state

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/db"
	"golang.org/x/exp/maps"
)

// contract storage has fixed height at 251
const (
	ContractStorageTrieHeight = 251
	contractDataSize          = 3*felt.Bytes + 8
)

var (
	ErrContractNotDeployed     = errors.New("contract not deployed")
	ErrContractAlreadyDeployed = errors.New("contract already deployed")
)

type Storage map[felt.Felt]*felt.Felt

type StateContract struct {
	// Hash of the contract's class
	ClassHash *felt.Felt
	// Contract's nonce
	Nonce *felt.Felt
	// Root hash of the contract's storage
	StorageRoot *felt.Felt
	// Height at which the contract is deployed
	DeployHeight uint64
	// Address that this contract instance is deployed to
	Address *felt.Felt
	// Storage locations that have been updated
	dirtyStorage Storage
	// The underlying storage trie
	tr *trie2.Trie
}

func NewStateContract(
	addr *felt.Felt,
	classHash *felt.Felt,
	nonce *felt.Felt,
	deployHeight uint64,
) *StateContract {
	contract := &StateContract{
		Address:      addr,
		ClassHash:    classHash,
		Nonce:        nonce,
		DeployHeight: deployHeight,
		dirtyStorage: make(Storage),
	}

	return contract
}

func (s *StateContract) GetStorageRoot(txn db.Transaction) (*felt.Felt, error) {
	if s.StorageRoot != nil {
		return s.StorageRoot, nil
	}

	tr, err := s.getTrie(txn)
	if err != nil {
		return nil, err
	}

	root := tr.Hash()
	s.StorageRoot = &root

	return &root, nil
}

func (s *StateContract) UpdateStorage(key, value *felt.Felt) {
	if s.dirtyStorage == nil {
		s.dirtyStorage = make(Storage)
	}

	s.dirtyStorage[*key] = value
}

func (s *StateContract) GetStorage(key *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
	if s.dirtyStorage != nil {
		if val, ok := s.dirtyStorage[*key]; ok {
			return val, nil
		}
	}

	var err error

	tr := s.tr
	if tr == nil {
		tr, err = s.getTrie(txn)
		if err != nil {
			return nil, err
		}
	}

	val, err := tr.Get(key)
	if err != nil {
		return nil, err
	}

	return &val, nil
}

// Marshals the contract into a byte slice
func (s *StateContract) MarshalBinary() ([]byte, error) {
	buf := make([]byte, contractDataSize)

	copy(buf[0:felt.Bytes], s.ClassHash.Marshal())
	copy(buf[felt.Bytes:2*felt.Bytes], s.Nonce.Marshal())
	copy(buf[2*felt.Bytes:3*felt.Bytes], s.StorageRoot.Marshal())
	binary.BigEndian.PutUint64(buf[3*felt.Bytes:contractDataSize], s.DeployHeight)

	return buf, nil
}

// Unmarshals the contract from a byte slice
func (s *StateContract) UnmarshalBinary(data []byte) error {
	if len(data) != contractDataSize {
		return fmt.Errorf("invalid length for StateContract: got %d, want %d", len(data), contractDataSize)
	}

	s.ClassHash = new(felt.Felt).SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]
	s.Nonce = new(felt.Felt).SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]
	s.StorageRoot = new(felt.Felt).SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]
	s.DeployHeight = binary.BigEndian.Uint64(data[:8])

	return nil
}

func (s *StateContract) Commit(txn db.Transaction, storeHistory bool, blockNum uint64) error {
	var err error

	tr := s.tr
	if tr == nil {
		tr, err = s.getTrie(txn)
		if err != nil {
			return err
		}
	}

	keys := maps.Keys(s.dirtyStorage)
	slices.SortFunc(keys, func(a, b felt.Felt) int {
		return b.Cmp(&a)
	})

	// Commit storage changes to the associated storage trie
	for _, key := range keys {
		val := s.dirtyStorage[key]
		if err := tr.Update(&key, val); err != nil {
			return err
		}

		if storeHistory {
			if err := s.storeStorageHistory(txn, blockNum, &key, val); err != nil {
				return err
			}
		}
	}

	root, err := tr.Commit()
	if err != nil {
		return err
	}
	s.StorageRoot = &root

	if storeHistory {
		if err := s.storeNonceHistory(txn, blockNum); err != nil {
			return err
		}

		if err := s.storeClassHashHistory(txn, blockNum); err != nil {
			return err
		}
	}

	return s.flush(txn)
}

// Calculates and returns the commitment of the contract
func (s *StateContract) Commitment() *felt.Felt {
	return crypto.Pedersen(crypto.Pedersen(crypto.Pedersen(s.ClassHash, s.StorageRoot), s.Nonce), &felt.Zero)
}

func (s *StateContract) storeNonceHistory(txn db.Transaction, blockNum uint64) error {
	keyBytes := contractHistoryNonceKey(s.Address, blockNum)
	return txn.Set(keyBytes, s.Nonce.Marshal())
}

func (s *StateContract) storeClassHashHistory(txn db.Transaction, blockNum uint64) error {
	keyBytes := contractHistoryClassHashKey(s.Address, blockNum)
	return txn.Set(keyBytes, s.ClassHash.Marshal())
}

func (s *StateContract) storeStorageHistory(txn db.Transaction, blockNum uint64, key, value *felt.Felt) error {
	keyBytes := contractHistoryStorageKey(s.Address, key, blockNum)
	return txn.Set(keyBytes, value.Marshal())
}

func (s *StateContract) delete(txn db.Transaction) error {
	key := contractKey(s.Address)
	return txn.Delete(key)
}

func (s *StateContract) deleteStorageTrie(txn db.Transaction) error {
	tr, err := s.getTrie(txn)
	if err != nil {
		return err
	}

	// TODO: Instead of using node iterator and delete each node one by one,
	// use the underlying DeleteRange from PebbleDB.
	it, err := tr.NodeIterator()
	if err != nil {
		return err
	}
	defer it.Close()

	for it.First(); it.Valid(); it.Next() {
		key := it.Key()
		if err := txn.Delete(key); err != nil {
			return err
		}
	}

	return nil
}

// Flush the contract to the database
func (s *StateContract) flush(txn db.Transaction) error {
	key := contractKey(s.Address)
	data, err := s.MarshalBinary()
	if err != nil {
		return err
	}

	return txn.Set(key, data)
}

func (s *StateContract) getTrie(txn db.Transaction) (*trie2.Trie, error) {
	if s.tr != nil {
		return s.tr, nil
	}

	var tr *trie2.Trie
	var err error
	//TODO(MaksymMalicki): handle for both db schemes
	if true {
		if s.StorageRoot != nil {
			tr, err = trie2.NewWithRootHash(trie2.NewContractStorageTrieID(*s.Address), ContractStorageTrieHeight, crypto.Pedersen, txn, *s.StorageRoot)
		} else {
			tr, err = trie2.New(trie2.NewContractStorageTrieID(*s.Address), ContractStorageTrieHeight, crypto.Pedersen, txn)
		}
	} else {
		tr, err = trie2.New(trie2.NewContractStorageTrieID(*s.Address), ContractStorageTrieHeight, crypto.Pedersen, txn)
	}
	if err != nil {
		return nil, err
	}
	s.tr = tr

	return tr, nil
}

// Wrapper around getContract which checks if a contract is deployed
func GetContract(addr *felt.Felt, txn db.Transaction) (*StateContract, error) {
	contract, err := getContract(addr, txn)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrContractNotDeployed
		}
		return nil, err
	}

	return contract, nil
}

// Gets a contract instance from the database.
func getContract(addr *felt.Felt, txn db.Transaction) (*StateContract, error) {
	key := contractKey(addr)
	var contract StateContract
	if err := txn.Get(key, func(val []byte) error {
		if err := contract.UnmarshalBinary(val); err != nil {
			return fmt.Errorf("failed to unmarshal contract: %w", err)
		}

		contract.Address = addr
		contract.dirtyStorage = make(Storage)

		return nil
	}); err != nil {
		return nil, err
	}
	return &contract, nil
}

func contractKey(addr *felt.Felt) []byte {
	return db.Contract.Key(addr.Marshal())
}

func contractHistoryNonceKey(addr *felt.Felt, blockNum uint64) []byte {
	return db.ContractNonceHistory.Key(addr.Marshal(), uint64ToBytes(blockNum))
}

func contractHistoryClassHashKey(addr *felt.Felt, blockNum uint64) []byte {
	return db.ContractClassHashHistory.Key(addr.Marshal(), uint64ToBytes(blockNum))
}

func contractHistoryStorageKey(addr, key *felt.Felt, blockNum uint64) []byte {
	return db.ContractStorageHistory.Key(addr.Marshal(), key.Marshal(), uint64ToBytes(blockNum))
}

func uint64ToBytes(num uint64) []byte {
	const size = 8
	buf := make([]byte, size)
	binary.BigEndian.PutUint64(buf, num)
	return buf
}
