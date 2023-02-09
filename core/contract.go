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

// Class unambiguously defines a [Contract]'s semantics.
type Class struct {
	// The version of the class, currently always 0.
	APIVersion *felt.Felt
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// An ascii-encoded array of builtin names imported by the class.
	Builtins []*felt.Felt
	// The starknet_keccak hash of the ".json" file compiler output.
	ProgramHash *felt.Felt
	Bytecode    []*felt.Felt
}

func (c *Class) Hash() *felt.Felt {
	return crypto.PedersenArray(
		c.APIVersion,
		crypto.PedersenArray(flatten(c.Externals)...),
		crypto.PedersenArray(flatten(c.L1Handlers)...),
		crypto.PedersenArray(flatten(c.Constructors)...),
		crypto.PedersenArray(c.Builtins...),
		c.ProgramHash,
		crypto.PedersenArray(c.Bytecode...),
	)
}

func flatten(entryPoints []EntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because it
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = entryPoint.Offset
	}
	return result
}

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt
}

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
	if _, err := c.txn.Get(classHashKey); err == nil {
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
func (c *Contract) Nonce() (*felt.Felt, error) {
	key := db.ContractNonce.Key(c.Address.Marshal())
	val, err := c.txn.Get(key)
	if err != nil {
		return nil, err
	}
	return new(felt.Felt).SetBytes(val), nil
}

// UpdateNonce updates the nonce value in the database.
func (c *Contract) UpdateNonce(nonce *felt.Felt) error {
	nonceKey := db.ContractNonce.Key(c.Address.Marshal())
	return c.txn.Set(nonceKey, nonce.Marshal())
}

// ClassHash returns hash of the class that this contract instantiates.
func (c *Contract) ClassHash() (*felt.Felt, error) {
	key := db.ContractClassHash.Key(c.Address.Marshal())
	val, err := c.txn.Get(key)
	if err != nil {
		return nil, err
	}
	return new(felt.Felt).SetBytes(val), nil
}

// Storage returns the [core.Trie] that represents the
// storage of the contract.
func (c *Contract) Storage() (*trie.Trie, error) {
	addrBytes := c.Address.Marshal()
	var contractRootKey *bitset.BitSet

	if val, err := c.txn.Get(db.ContractRootKey.Key(addrBytes)); err == nil {
		contractRootKey = new(bitset.BitSet)
		if err = contractRootKey.UnmarshalBinary(val); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		// Don't continue normal operation with arbitrary
		// database error.
		return nil, err
	}
	trieTxn := trie.NewTrieTxn(c.txn, db.ContractStorage.Key(addrBytes))
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
