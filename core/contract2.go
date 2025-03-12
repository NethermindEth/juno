package core

import (
	"errors"

	// "github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

// contract storage has fixed height at 251
// const ContractStorageTrieHeight = 251

// var (
// 	ErrContractNotDeployed     = errors.New("contract not deployed")
// 	ErrContractAlreadyDeployed = errors.New("contract already deployed")
// )

// NewContractUpdater2 creates an updater for the contract instance at the given address.
// Deploy should be called for contracts that were just deployed to the network.
func NewContractUpdater2(addr *felt.Felt, txn db.IndexedBatch) (*ContractUpdater2, error) {
	contractDeployed, err := deployed2(addr, txn)
	if err != nil {
		return nil, err
	}

	if !contractDeployed {
		return nil, ErrContractNotDeployed
	}

	return &ContractUpdater2{
		Address: addr,
		txn:     txn,
	}, nil
}

// DeployContract sets up the database for a new contract.
func DeployContract2(addr, classHash *felt.Felt, txn db.IndexedBatch) (*ContractUpdater2, error) {
	contractDeployed, err := deployed2(addr, txn)
	if err != nil {
		return nil, err
	}

	if contractDeployed {
		return nil, ErrContractAlreadyDeployed
	}

	err = setClassHash2(txn, addr, classHash)
	if err != nil {
		return nil, err
	}

	c, err := NewContractUpdater2(addr, txn)
	if err != nil {
		return nil, err
	}

	err = c.UpdateNonce(&felt.Zero)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// // ContractAddress computes the address of a Starknet contract.
// func ContractAddress(callerAddress, classHash, salt *felt.Felt, constructorCallData []*felt.Felt) *felt.Felt {
// 	prefix := new(felt.Felt).SetBytes([]byte("STARKNET_CONTRACT_ADDRESS"))
// 	callDataHash := crypto.PedersenArray(constructorCallData...)

// 	// https://docs.starknet.io/architecture-and-concepts/smart-contracts/contract-address/
// 	return crypto.PedersenArray(
// 		prefix,
// 		callerAddress,
// 		salt,
// 		classHash,
// 		callDataHash,
// 	)
// }

func deployed2(addr *felt.Felt, txn db.IndexedBatch) (bool, error) {
	_, err := ContractClassHash2(addr, txn)
	if errors.Is(err, db.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ContractUpdater2 is a helper to update an existing contract instance.
type ContractUpdater2 struct {
	// Address that this contract instance is deployed to
	Address *felt.Felt
	// txn to access the database
	txn db.IndexedBatch
}

// Purge eliminates the contract instance, deleting all associated data from storage
// assumes storage is cleared in revert process
func (c *ContractUpdater2) Purge() error {
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
func ContractNonce2(addr *felt.Felt, txn db.IndexedBatch) (*felt.Felt, error) {
	key := db.ContractNonceKey(addr)
	val, err := txn.Get2(key)
	if err != nil {
		return nil, err
	}
	return new(felt.Felt).SetBytes(val), nil
}

// UpdateNonce updates the nonce value in the database.
func (c *ContractUpdater2) UpdateNonce(nonce *felt.Felt) error {
	nonceKey := db.ContractNonceKey(c.Address)
	return c.txn.Put(nonceKey, nonce.Marshal())
}

// ContractRoot returns the root of the contract storage.
func ContractRoot2(addr *felt.Felt, txn db.IndexedBatch) (*felt.Felt, error) {
	cStorage, err := storage2(addr, txn)
	if err != nil {
		return nil, err
	}
	return cStorage.Root()
}

// type OnValueChanged = func(location, oldValue *felt.Felt) error

// UpdateStorage applies a change-set to the contract storage.
func (c *ContractUpdater2) UpdateStorage(diff map[felt.Felt]*felt.Felt, cb OnValueChanged) error {
	cStorage, err := storage2(c.Address, c.txn)
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

func ContractStorage2(addr, key *felt.Felt, txn db.IndexedBatch) (*felt.Felt, error) {
	cStorage, err := storage2(addr, txn)
	if err != nil {
		return nil, err
	}
	return cStorage.Get(key)
}

// ContractClassHash returns hash of the class that the contract at the given address instantiates.
func ContractClassHash2(addr *felt.Felt, txn db.IndexedBatch) (*felt.Felt, error) {
	key := db.ContractClassHashKey(addr)
	val, err := txn.Get2(key)
	if err != nil {
		return nil, err
	}
	return new(felt.Felt).SetBytes(val), nil
}

func setClassHash2(txn db.IndexedBatch, addr, classHash *felt.Felt) error {
	classHashKey := db.ContractClassHashKey(addr)
	return txn.Put(classHashKey, classHash.Marshal())
}

// Replace replaces the class that the contract instantiates
func (c *ContractUpdater2) Replace(classHash *felt.Felt) error {
	return setClassHash2(c.txn, c.Address, classHash)
}

// storage returns the [core.Trie] that represents the
// storage of the contract.
func storage2(addr *felt.Felt, txn db.IndexedBatch) (*trie.Trie2, error) {
	addrBytes := addr.Marshal()
	trieTxn := trie.NewStorage2(txn, db.ContractStorage.Key(addrBytes))
	return trie.NewTriePedersen2(trieTxn, ContractStorageTrieHeight)
}
