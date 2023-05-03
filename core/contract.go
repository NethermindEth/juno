package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
)

const contractStorageTrieHeight = 251

var (
	ErrContractNotDeployed     = errors.New("contract not deployed")
	ErrContractAlreadyDeployed = errors.New("contract already deployed")
)

// NewContract creates a contract instance at the given address.
// Deploy should be called for contracts that were just deployed to the network.
func NewContract(addr *felt.Felt, txn db.Transaction) (*Contract, error) {
	contractDeployed, err := deployed(addr, txn)
	if err != nil {
		return nil, err
	}

	if !contractDeployed {
		return nil, ErrContractNotDeployed
	}

	return &Contract{
		Address: addr,
		txn:     txn,
	}, nil
}

// DeployContract sets up the database for a new contract.
func DeployContract(addr, classHash *felt.Felt, txn db.Transaction) (*Contract, error) {
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

	c, err := NewContract(addr, txn)
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
	_, err := classHash(addr, txn)
	if errors.Is(err, db.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Contract is an instance of a [Class].
type Contract struct {
	// Address that this contract instance is deployed to
	Address *felt.Felt
	// txn to access the database
	txn db.Transaction
}

// Nonce returns the amount transactions sent from this contract.
// Only account contracts can have a non-zero nonce.
func (c *Contract) Nonce() (*felt.Felt, error) {
	key := db.ContractNonce.Key(c.Address.Marshal())
	var nonce *felt.Felt
	if err := c.txn.Get(key, func(val []byte) error {
		nonce = new(felt.Felt)
		nonce.SetBytes(val)
		return nil
	}); err != nil {
		return nil, err
	}
	return nonce, nil
}

// UpdateNonce updates the nonce value in the database.
func (c *Contract) UpdateNonce(nonce *felt.Felt) error {
	nonceKey := db.ContractNonce.Key(c.Address.Marshal())
	return c.txn.Set(nonceKey, nonce.Marshal())
}

// ClassHash returns hash of the class that this contract instantiates.
func (c *Contract) ClassHash() (*felt.Felt, error) {
	return classHash(c.Address, c.txn)
}

// Root returns the root of the contract storage.
func (c *Contract) Root() (*felt.Felt, error) {
	cStorage, err := storage(c.Address, c.txn)
	if err != nil {
		return nil, err
	}
	return cStorage.Root()
}

type OnValueChanged = func(location, oldValue *felt.Felt) error

// UpdateStorage applies a change-set to the contract storage.
func (c *Contract) UpdateStorage(diff []StorageDiff, cb OnValueChanged) error {
	cStorage, err := storage(c.Address, c.txn)
	if err != nil {
		return err
	}
	// apply the diff
	for _, pair := range diff {
		oldValue, pErr := cStorage.Put(pair.Key, pair.Value)
		if pErr != nil {
			return pErr
		}

		if oldValue != nil {
			if err = cb(pair.Key, oldValue); err != nil {
				return err
			}
		}
	}

	if err = cStorage.Commit(); err != nil {
		return err
	}

	// update contract storage root in the database
	rootKeyDBKey := db.ContractRootKey.Key(c.Address.Marshal())
	if rootKey := cStorage.RootKey(); rootKey != nil {
		rootKeyBytes, err := rootKey.MarshalBinary()
		if err != nil {
			return err
		}

		if err := c.txn.Set(rootKeyDBKey, rootKeyBytes); err != nil {
			return err
		}
	} else if err := c.txn.Delete(rootKeyDBKey); err != nil {
		return err
	}

	return nil
}

func (c *Contract) Storage(key *felt.Felt) (*felt.Felt, error) {
	cStorage, err := storage(c.Address, c.txn)
	if err != nil {
		return nil, err
	}
	return cStorage.Get(key)
}

// ClassHash returns hash of the class that the contract at the given address instantiates.
func classHash(addr *felt.Felt, txn db.Transaction) (*felt.Felt, error) {
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
func (c *Contract) Replace(classHash *felt.Felt) error {
	return setClassHash(c.txn, c.Address, classHash)
}

// storage returns the [core.Trie] that represents the
// storage of the contract.
func storage(addr *felt.Felt, txn db.Transaction) (*trie.Trie, error) {
	addrBytes := addr.Marshal()
	var contractRootKey *bitset.BitSet

	if err := txn.Get(db.ContractRootKey.Key(addrBytes), func(val []byte) error {
		contractRootKey = new(bitset.BitSet)
		return contractRootKey.UnmarshalBinary(val)
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		// Don't continue normal operation with arbitrary
		// database error.
		return nil, err
	}
	trieTxn := trie.NewTransactionStorage(txn, db.ContractStorage.Key(addrBytes))
	return trie.NewTriePedersen(trieTxn, contractStorageTrieHeight, contractRootKey)
}
