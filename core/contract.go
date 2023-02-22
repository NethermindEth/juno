package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
)

const (
	contractStorageTrieHeight = 251
)

// Contract is an instance of a [Class].
type Contract struct {
	// Address that this contract instance is deployed to
	Address *felt.Felt
	// txn to access the database
	txn db.Transaction
}

// NewContract creates a contract instance at the given address.
// Deploy should be called for contracts that were just deployed to the network.
func NewContract(addr *felt.Felt, txn db.Transaction) *Contract {
	return &Contract{
		Address: addr,
		txn:     txn,
	}
}

// Deploy sets up the database for a new contract.
func (c *Contract) Deploy(classHash *felt.Felt) error {
	classHashKey := db.ContractClassHash.Key(c.Address.Marshal())
	if err := c.txn.Get(classHashKey, func(val []byte) error {
		return nil
	}); err == nil {
		// Should not happen.
		return errors.New("existing contract")
	} else if err = c.txn.Set(classHashKey, classHash.Marshal()); err != nil {
		return err
	} else if err = c.UpdateNonce(&felt.Zero); err != nil {
		return err
	}

	return nil
}

// Nonce returns the number of transactions sent from this contract.
// Only account contracts can have a non-zero nonce.
func (c *Contract) Nonce() (nonce *felt.Felt, err error) {
	key := db.ContractNonce.Key(c.Address.Marshal())
	err = c.txn.Get(key, func(val []byte) error {
		nonce = new(felt.Felt)
		nonce.SetBytes(val)
		return nil
	})
	return
}

// UpdateNonce updates the nonce value in the database.
func (c *Contract) UpdateNonce(nonce *felt.Felt) error {
	nonceKey := db.ContractNonce.Key(c.Address.Marshal())
	return c.txn.Set(nonceKey, nonce.Marshal())
}

// ClassHash returns hash of the class that this contract instantiates.
func (c *Contract) ClassHash() (classHash *felt.Felt, err error) {
	key := db.ContractClassHash.Key(c.Address.Marshal())
	err = c.txn.Get(key, func(val []byte) error {
		classHash = new(felt.Felt)
		classHash.SetBytes(val)
		return nil
	})
	return
}

// Storage returns the [core.Trie] that represents the
// storage of the contract.
func (c *Contract) Storage() (*trie.Trie, error) {
	addrBytes := c.Address.Marshal()
	var contractRootKey *bitset.BitSet

	if err := c.txn.Get(db.ContractRootKey.Key(addrBytes), func(val []byte) error {
		contractRootKey = new(bitset.BitSet)
		return contractRootKey.UnmarshalBinary(val)
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		// Don't continue normal operation with arbitrary
		// database error.
		return nil, err
	}
	trieTxn := NewTransactionStorage(c.txn, db.ContractStorage.Key(addrBytes))
	return trie.NewTrie(trieTxn, contractStorageTrieHeight, contractRootKey), nil
}

// StorageRoot returns the root of the contract storage.
func (c *Contract) StorageRoot() (*felt.Felt, error) {
	if storage, err := c.Storage(); err != nil {
		return nil, err
	} else {
		return storage.Root()
	}
}

// UpdateStorage applies a change-set to the contract storage.
func (c *Contract) UpdateStorage(diff []StorageDiff) error {
	storage, err := c.Storage()
	if err != nil {
		return err
	}

	// apply the diff
	for _, pair := range diff {
		if _, err = storage.Put(pair.Key, pair.Value); err != nil {
			return err
		}
	}

	// update contract storage root in the database
	rootKeyDbKey := db.ContractRootKey.Key(c.Address.Marshal())
	if rootKey := storage.RootKey(); rootKey != nil {
		if rootKeyBytes, err := rootKey.MarshalBinary(); err != nil {
			return err
		} else if err = c.txn.Set(rootKeyDbKey, rootKeyBytes); err != nil {
			return err
		}
	} else if err = c.txn.Delete(rootKeyDbKey); err != nil {
		return err
	}

	return nil
}

// ContractAddress computes the address of a Starknet contract.
func ContractAddress(callerAddress, classHash, salt *felt.Felt, constructorCallData []*felt.Felt) *felt.Felt {
	prefix := new(felt.Felt).SetBytes([]byte("STARKNET_CONTRACT_ADDRESS"))
	callDataHash := crypto.PedersenArray(constructorCallData...)

	// https://docs.starknet.io/documentation/architecture_and_concepts/Contracts/contract-address
	return crypto.PedersenArray(
		prefix,
		callerAddress,
		salt,
		classHash,
		callDataHash,
	)
}
