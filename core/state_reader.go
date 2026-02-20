package core

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state/commontrie"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

var _ StateHistoryReader = (*StateSnapshotReader)(nil)

//go:generate mockgen -destination=../mocks/mock_state.go -package=mocks github.com/NethermindEth/juno/core StateHistoryReader
type StateHistoryReader interface {
	StateReader

	ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractNonceAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (felt.Felt, error)
	ContractDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error)
	CompiledClassHashAt(
		classHash *felt.SierraClassHash,
		blockNumber uint64,
	) (felt.CasmClassHash, error)
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClassDefinition, error)
	CompiledClassHash(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	CompiledClassHashV2(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	ClassTrie() (commontrie.Trie, error)
	ContractTrie() (commontrie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error)
}

type StateSnapshotReader struct {
	txn db.KeyValueReader
}

func NewStateSnapshotReader(txn db.KeyValueReader) *StateSnapshotReader {
	return &StateSnapshotReader{
		txn: txn,
	}
}

// Root returns the state commitment.
func (s *StateSnapshotReader) Commitment() (felt.Felt, error) {
	var storageRoot, classesRoot felt.Felt

	sStorage, err := contractTrieReader(s.txn)
	if err != nil {
		return felt.Felt{}, err
	}

	storageRoot, err = sStorage.Hash()
	if err != nil {
		return felt.Felt{}, err
	}

	classes, err := classesTrieReader(s.txn)
	if err != nil {
		return felt.Felt{}, err
	}

	classesRoot, err = classes.Hash()
	if err != nil {
		return felt.Felt{}, err
	}

	if classesRoot.IsZero() {
		return storageRoot, nil
	}

	root := crypto.PoseidonArray(stateVersion, &storageRoot, &classesRoot)
	return root, nil
}

// ContractStorageAt returns the value of a storage location
// of the given contract at the height `height`
func (s *StateSnapshotReader) ContractStorageAt(
	contractAddress,
	storageLocation *felt.Felt,
	height uint64,
) (felt.Felt, error) {
	key := db.ContractStorageHistoryKey(contractAddress, storageLocation)
	value, err := s.valueAt(key, height)
	if err != nil {
		return felt.Felt{}, err
	}

	return felt.FromBytes[felt.Felt](value), nil
}

func (s *StateSnapshotReader) ContractNonceAt(
	contractAddress *felt.Felt,
	height uint64,
) (felt.Felt, error) {
	key := db.ContractNonceHistoryKey(contractAddress)
	value, err := s.valueAt(key, height)
	if err != nil {
		return felt.Felt{}, err
	}
	return felt.FromBytes[felt.Felt](value), nil
}

func (s *StateSnapshotReader) ContractClassHashAt(
	contractAddress *felt.Felt,
	height uint64,
) (felt.Felt, error) {
	key := db.ContractClassHashHistoryKey(contractAddress)
	value, err := s.valueAt(key, height)
	if err != nil {
		return felt.Felt{}, err
	}

	return felt.FromBytes[felt.Felt](value), nil
}

// ContractDeployedAt returns if contract at given addr was deployed at blockNumber
func (s *StateSnapshotReader) ContractDeployedAt(
	addr *felt.Felt,
	blockNumber uint64,
) (bool, error) {
	var deployedAt uint64

	err := s.txn.Get(db.ContractDeploymentHeightKey(addr), func(data []byte) error {
		deployedAt = binary.BigEndian.Uint64(data)
		return nil
	})
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}

	return deployedAt <= blockNumber, nil
}

func (s *StateSnapshotReader) CompiledClassHashAt(
	classHash *felt.SierraClassHash,
	blockNumber uint64,
) (felt.CasmClassHash, error) {
	metadata, err := GetClassCasmHashMetadata(s.txn, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHashAt(blockNumber)
}

// Class returns the class object corresponding to the given classHash
func (s *StateSnapshotReader) Class(classHash *felt.Felt) (*DeclaredClassDefinition, error) {
	var class *DeclaredClassDefinition
	err := s.txn.Get(db.ClassKey(classHash), func(data []byte) error {
		return encoder.Unmarshal(data, &class)
	})
	return class, err
}

// ContractClassHash returns class hash of a contract at a given address.
func (s *StateSnapshotReader) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	return ContractClassHash(addr, s.txn)
}

// ContractNonce returns nonce of a contract at a given address.
func (s *StateSnapshotReader) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	return ContractNonce(addr, s.txn)
}

// ContractStorage returns value of a key in the storage of the contract at the given address.
func (s *StateSnapshotReader) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	return ContractStorage(addr, key, s.txn)
}

func (s *StateSnapshotReader) ClassTrie() (commontrie.Trie, error) {
	// We don't need to call the closer function here because we are only reading the trie
	tr, err := classesTrieReader(s.txn)
	return tr, err
}

func (s *StateSnapshotReader) ContractTrie() (commontrie.Trie, error) {
	tr, err := contractTrieReader(s.txn)
	return tr, err
}

func (s *StateSnapshotReader) ContractStorageTrie(addr *felt.Felt) (commontrie.Trie, error) {
	return storageReader(addr, s.txn)
}

func (s *StateSnapshotReader) valueAt(key []byte, height uint64) ([]byte, error) {
	it, err := s.txn.NewIterator(nil, false)
	if err != nil {
		return nil, err
	}

	seekKey := binary.BigEndian.AppendUint64(key, height)
	for it.Seek(seekKey); it.Valid(); it.Next() {
		seekedKey := it.Key()
		// seekedKey size should be `len(key) + sizeof(uint64)` and seekedKey should match key prefix
		if len(seekedKey) != len(key)+8 || !bytes.HasPrefix(seekedKey, key) {
			break
		}

		seekedHeight := binary.BigEndian.Uint64(seekedKey[len(key):])
		if seekedHeight < height {
			// last change happened before the height we are looking for
			// check head state
			break
		} else if seekedHeight == height {
			// a log exists for the height we are looking for, so the old value in this
			// log entry is not useful. Advance the iterator and see we can use the next entry.
			// If not, ErrCheckHeadState will be returned.
			continue
		}

		val, itErr := it.Value()
		if err = utils.RunAndWrapOnError(it.Close, itErr); err != nil {
			return nil, err
		}
		// seekedHeight > height
		return val, nil
	}

	return nil, utils.RunAndWrapOnError(it.Close, ErrCheckHeadState)
}

func (s *StateSnapshotReader) GetReverseStateDiff(
	blockNumber uint64,
	diff *StateDiff,
) (StateDiff, error) {
	reversed := *diff

	reversed.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs))
	for addr, storageDiffs := range diff.StorageDiffs {
		reversedDiffs := make(map[felt.Felt]*felt.Felt, len(storageDiffs))
		for key := range storageDiffs {
			value := felt.Zero
			if blockNumber > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNumber-1)
				if err != nil {
					return StateDiff{}, err
				}
				value = oldValue
			}
			reversedDiffs[key] = &value
		}
		reversed.StorageDiffs[addr] = reversedDiffs
	}

	reversed.Nonces = make(map[felt.Felt]*felt.Felt, len(diff.Nonces))
	for addr := range diff.Nonces {
		oldNonce := felt.Zero
		if blockNumber > 0 {
			var err error
			oldNonce, err = s.ContractNonceAt(&addr, blockNumber-1)
			if err != nil {
				return StateDiff{}, err
			}
		}
		reversed.Nonces[addr] = &oldNonce
	}

	reversed.ReplacedClasses = make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses))
	for addr := range diff.ReplacedClasses {
		classHash := felt.Zero
		if blockNumber > 0 {
			var err error
			classHash, err = s.ContractClassHashAt(&addr, blockNumber-1)
			if err != nil {
				return StateDiff{}, err
			}
		}
		reversed.ReplacedClasses[addr] = &classHash
	}

	return reversed, nil
}

func (s *StateSnapshotReader) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	metadata, err := GetClassCasmHashMetadata(s.txn, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHash(), nil
}

func (s *StateSnapshotReader) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	metadata, err := GetClassCasmHashMetadata(s.txn, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHashV2(), nil
}

// storage returns a [core.Trie] that represents the Starknet global state in the given Txn context.
func contractTrieReader(txn db.KeyValueReader) (*trie.TrieReader, error) {
	return globalTrieReader(txn, db.StateTrie, trie.NewTrieReaderPedersen)
}

func classesTrieReader(txn db.KeyValueReader) (*trie.TrieReader, error) {
	return globalTrieReader(txn, db.ClassesTrie, trie.NewTrieReaderPoseidon)
}

func globalTrieReader(
	txn db.KeyValueReader,
	bucket db.Bucket,
	newTrieReader trie.NewTrieReaderFunc,
) (*trie.TrieReader, error) {
	dbPrefix := bucket.Key()

	// fetch root key
	rootKeyDBKey := dbPrefix
	var val []byte
	err := txn.Get(rootKeyDBKey, func(data []byte) error {
		val = data
		return nil
	})
	// if some error other than "not found"
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	rootKey := new(trie.BitArray)
	if len(val) > 0 {
		err = rootKey.UnmarshalBinary(val)
		if err != nil {
			return nil, err
		}
	}

	gTrie, err := newTrieReader(txn, dbPrefix, globalTrieHeight)
	if err != nil {
		return nil, err
	}

	return gTrie, nil
}
