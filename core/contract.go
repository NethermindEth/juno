package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

// contract storage has fixed height at 251
const ContractStorageTrieHeight = 251

var (
	ErrContractNotDeployed     = errors.New("contract not deployed")
	ErrContractAlreadyDeployed = errors.New("contract already deployed")
)

type OnValueChanged = func(location, oldValue *felt.Felt) error

type StateContract struct {
	// ClassHash is the hash of the contract's class
	ClassHash *felt.Felt
	// Nonce is the contract's nonce
	Nonce *felt.Felt
	// DeployHeight is the height at which the contract is deployed
	DeployHeight uint64
	// Address that this contract instance is deployed to
	Address *felt.Felt `cbor:"-"`
	// Storage is the contract's storage
	Storage map[felt.Felt]*felt.Felt `cbor:"-"`
}

func NewStateContract(
	addr *felt.Felt,
	classHash *felt.Felt,
	nonce *felt.Felt,
	storage map[felt.Felt]*felt.Felt,
	DeployHeight uint64,
) *StateContract {
	sc := &StateContract{
		Address:      addr,
		ClassHash:    classHash,
		Nonce:        nonce,
		Storage:      storage,
		DeployHeight: DeployHeight,
	}

	if storage == nil {
		sc.Storage = make(map[felt.Felt]*felt.Felt)
	}

	return sc
}

func (c *StateContract) StorageRoot(txn db.Transaction) (*felt.Felt, error) {
	storageTrie, err := storage(c.Address, txn)
	if err != nil {
		return nil, err
	}

	return storageTrie.Root()
}

func (c *StateContract) GetStorage(key *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
	if c.Storage != nil {
		if val, ok := c.Storage[*key]; ok {
			return val, nil
		}
	}

	// get from db
	storage, err := storage(c.Address, txn)
	if err != nil {
		return nil, err
	}

	return storage.Get(key)
}

func (c *StateContract) Commit(txn db.Transaction, cb OnValueChanged) error {
	storageTrie, err := storage(c.Address, txn)
	if err != nil {
		return err
	}

	for key, value := range c.Storage {
		oldVal, err := storageTrie.Put(&key, value)
		if err != nil {
			return err
		}

		if oldVal != nil {
			if err = cb(&key, oldVal); err != nil {
				return err
			}
		}
	}

	if err := storageTrie.Commit(); err != nil {
		return err
	}

	contractBytes, err := encoder.Marshal(c)
	if err != nil {
		return err
	}

	return txn.Set(db.Contract.Key(c.Address.Marshal()), contractBytes)
}

// Purge eliminates the contract instance, deleting all associated data from database
// assumes storage is cleared in revert process
func (c *StateContract) Purge(txn db.Transaction) error {
	addrBytes := c.Address.Marshal()

	if err := txn.Delete(db.Contract.Key(addrBytes)); err != nil {
		return err
	}

	return txn.Delete(db.ContractDeploymentHeight.Key(addrBytes))
}

// GetContract is a wrapper around getContract which checks if a contract is deployed
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

// getContract gets a contract instance from the database.
func getContract(addr *felt.Felt, txn db.Transaction) (*StateContract, error) {
	key := db.Contract.Key(addr.Marshal())
	var contract StateContract
	if err := txn.Get(key, func(val []byte) error {
		if err := encoder.Unmarshal(val, &contract); err != nil {
			return err
		}

		contract.Address = addr
		contract.Storage = make(map[felt.Felt]*felt.Felt)

		return nil
	}); err != nil {
		return nil, err
	}
	return &contract, nil
}

// ContractAddress computes the address of a Starknet contract.
func ContractAddress(callerAddress, classHash, salt *felt.Felt, constructorCallData []*felt.Felt) *felt.Felt {
	prefix := new(felt.Felt).SetBytes([]byte("STARKNET_CONTRACT_ADDRESS"))
	callDataHash := crypto.PedersenArray(constructorCallData...)

	// https://docs.starknet.io/architecture-and-concepts/smart-contracts/contract-address/
	return crypto.PedersenArray(
		prefix,
		callerAddress,
		salt,
		classHash,
		callDataHash,
	)
}

// storage returns the [core.Trie] that represents the
// storage of the contract.
// TODO(weiihann): how to deal with the root key?
func storage(addr *felt.Felt, txn db.Transaction) (*trie.Trie, error) {
	addrBytes := addr.Marshal()
	trieTxn := trie.NewStorage(txn, db.ContractStorage.Key(addrBytes))
	return trie.NewTriePedersen(trieTxn, ContractStorageTrieHeight)
}
