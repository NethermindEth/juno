package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

const contractStorageTrieHeight = 251

var (
	ErrContractNotDeployed     = errors.New("contract not deployed")
	ErrContractAlreadyDeployed = errors.New("contract already deployed")
)

// NewContractUpdater creates an updater for the contract instance at the given address.
// Deploy should be called for contracts that were just deployed to the network.
func NewContractUpdater(addr *felt.Felt, txn db.Transaction) (*ContractUpdater, error) {
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
func DeployContract(addr, classHash *felt.Felt, txn db.Transaction) (*ContractUpdater, error) {
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

func deployed(addr *felt.Felt, txn db.Transaction) (bool, error) {
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
	txn db.Transaction
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
func ContractNonce(addr *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
	key := db.ContractNonce.Key(addr.Marshal())
	var nonce *felt.Felt
	if err := txn.Get(key, func(val []byte) error {
		nonce = new(felt.Felt)
		nonce.SetBytes(val)
		return nil
	}); err != nil {
		return nil, err
	}
	return nonce, nil
}

// UpdateNonce updates the nonce value in the database.
func (c *ContractUpdater) UpdateNonce(nonce *felt.Felt) error {
	nonceKey := db.ContractNonce.Key(c.Address.Marshal())
	return c.txn.Set(nonceKey, nonce.Marshal())
}

// ContractRoot returns the root of the contract storage.
func ContractRoot(addr *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
	cStorage, err := storage(addr, txn)
	if err != nil {
		return nil, err
	}
	return cStorage.Root()
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

	if err = cStorage.Commit(); err != nil {
		return err
	}

	return nil
}

func ContractStorage(addr, key *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
	cStorage, err := storage(addr, txn)
	if err != nil {
		return nil, err
	}
	return cStorage.Get(key)
}

// ContractClassHash returns hash of the class that the contract at the given address instantiates.
func ContractClassHash(addr *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
	key := db.ContractClassHash.Key(addr.Marshal())
	var classHash *felt.Felt
	if err := txn.Get(key, func(val []byte) error {
		classHash = new(felt.Felt)
		classHash.SetBytes(val)
		return nil
	}); err != nil {
		return nil, err
	}
	return classHash, nil
}

func setClassHash(txn db.Transaction, addr, classHash *felt.Felt) error {
	classHashKey := db.ContractClassHash.Key(addr.Marshal())
	return txn.Set(classHashKey, classHash.Marshal())
}

// Replace replaces the class that the contract instantiates
func (c *ContractUpdater) Replace(classHash *felt.Felt) error {
	return setClassHash(c.txn, c.Address, classHash)
}

// storage returns the [core.Trie] that represents the
// storage of the contract.
func storage(addr *felt.Felt, txn db.Transaction) (*trie.Trie, error) {
	addrBytes := addr.Marshal()
	trieTxn := trie.NewTransactionStorage(txn, db.ContractStorage.Key(addrBytes))
	return trie.NewTriePedersen(trieTxn, contractStorageTrieHeight)
}
