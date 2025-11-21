package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

// contract storage has fixed height at 251
const ContractStorageTrieHeight = 251

var (
	ErrContractNotDeployed     = errors.New("contract not deployed")
	ErrContractAlreadyDeployed = errors.New("contract already deployed")
)

// NewContractUpdater creates an updater for the contract instance at the given address.
// Deploy should be called for contracts that were just deployed to the network.
func NewContractUpdater(addr *felt.Felt, txn db.IndexedBatch) (*ContractUpdater, error) {
	contractDeployed, err := deployed(addr, txn)
	if err != nil {
		return nil, err
	}

	if !contractDeployed {
		return nil, ErrContractNotDeployed
	}

	return &ContractUpdater{
		Address: addr,
		txn:     txn,
	}, nil
}

// DeployContract sets up the database for a new contract.
func DeployContract(addr, classHash *felt.Felt, txn db.IndexedBatch) (*ContractUpdater, error) {
	contractDeployed, err := deployed(addr, txn)
	if err != nil {
		return nil, err
	}

	if contractDeployed {
		return nil, ErrContractAlreadyDeployed
	}

	err = setClassHash(txn, addr, classHash)
	if err != nil {
		return nil, err
	}

	c, err := NewContractUpdater(addr, txn)
	if err != nil {
		return nil, err
	}

	err = c.UpdateNonce(&felt.Zero)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// ContractAddress computes the address of a Starknet contract.
func ContractAddress(
	callerAddress,
	classHash,
	salt *felt.Felt,
	constructorCallData []*felt.Felt,
) felt.Felt {
	prefix := felt.FromBytes[felt.Felt]([]byte("STARKNET_CONTRACT_ADDRESS"))
	callDataHash := crypto.PedersenArray(constructorCallData...)

	// https://docs.starknet.io/architecture-and-concepts/smart-contracts/contract-address/
	return crypto.PedersenArray(
		&prefix,
		callerAddress,
		salt,
		classHash,
		&callDataHash,
	)
}

func deployed(addr *felt.Felt, txn db.IndexedBatch) (bool, error) {
	_, err := ContractClassHash(addr, txn)
	if errors.Is(err, db.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ContractUpdater is a helper to update an existing contract instance.
type ContractUpdater struct {
	// Address that this contract instance is deployed to
	Address *felt.Felt
	// txn to access the database
	txn db.IndexedBatch
}

// Purge eliminates the contract instance, deleting all associated data from storage
// assumes storage is cleared in revert process
func (c *ContractUpdater) Purge() error {
	addrBytes := c.Address.Marshal()
	buckets := []db.Bucket{db.ContractNonce, db.ContractClassHash}

	for _, bucket := range buckets {
		if err := c.txn.Delete(bucket.Key(addrBytes)); err != nil {
			return err
		}
	}

	return nil
}

// ContractNonce returns the amount transactions sent from this contract.
// Only account contracts can have a non-zero nonce.
func ContractNonce(addr *felt.Felt, txn db.KeyValueReader) (felt.Felt, error) {
	return GetContractNonce(txn, addr)
}

// UpdateNonce updates the nonce value in the database.
func (c *ContractUpdater) UpdateNonce(nonce *felt.Felt) error {
	nonceKey := db.ContractNonceKey(c.Address)
	return c.txn.Put(nonceKey, nonce.Marshal())
}

// ContractRoot returns the root of the contract storage.
func ContractRoot(addr *felt.Felt, txn db.IndexedBatch) (felt.Felt, error) {
	cStorage, err := storage(addr, txn)
	if err != nil {
		return felt.Felt{}, err
	}
	return cStorage.Hash()
}

type OnValueChanged = func(location, oldValue *felt.Felt) error

// UpdateStorage applies a change-set to the contract storage.
func (c *ContractUpdater) UpdateStorage(diff map[felt.Felt]*felt.Felt, cb OnValueChanged) error {
	cStorage, err := storage(c.Address, c.txn)
	if err != nil {
		return err
	}
	// apply the diff
	for key, value := range diff {
		oldValue, pErr := cStorage.Put(&key, value)
		if pErr != nil {
			return pErr
		}

		if oldValue != nil {
			if err = cb(&key, oldValue); err != nil {
				return err
			}
		}
	}

	return cStorage.Commit()
}

func ContractStorage(addr, key *felt.Felt, txn db.IndexedBatch) (felt.Felt, error) {
	cStorage, err := storage(addr, txn)
	if err != nil {
		return felt.Felt{}, err
	}
	return cStorage.Get(key)
}

// ContractClassHash returns hash of the class that the contract at the given address instantiates.
func ContractClassHash(addr *felt.Felt, txn db.KeyValueReader) (felt.Felt, error) {
	return GetContractClassHash(txn, addr)
}

func setClassHash(txn db.IndexedBatch, addr, classHash *felt.Felt) error {
	classHashKey := db.ContractClassHashKey(addr)
	return txn.Put(classHashKey, classHash.Marshal())
}

// Replace replaces the class that the contract instantiates
func (c *ContractUpdater) Replace(classHash *felt.Felt) error {
	return setClassHash(c.txn, c.Address, classHash)
}

// storage returns the [core.Trie] that represents the
// storage of the contract.
func storage(addr *felt.Felt, txn db.IndexedBatch) (*trie.Trie, error) {
	addrBytes := addr.Marshal()
	return trie.NewTriePedersen(txn, db.ContractStorage.Key(addrBytes), ContractStorageTrieHeight)
}

func storageReader(addr *felt.Felt, txn db.KeyValueReader) (*trie.TrieReader, error) {
	addrBytes := addr.Marshal()
	return trie.NewTrieReaderPedersen(txn, db.ContractStorage.Key(addrBytes), ContractStorageTrieHeight)
}
