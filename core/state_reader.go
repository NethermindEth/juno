package core

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
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
}

type StateReader interface {
	ContractClassHash(addr *felt.Felt) (felt.Felt, error)
	ContractNonce(addr *felt.Felt) (felt.Felt, error)
	ContractStorage(addr, key *felt.Felt) (felt.Felt, error)
	Class(classHash *felt.Felt) (*DeclaredClassDefinition, error)
	CompiledClassHash(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	CompiledClassHashV2(classHash *felt.SierraClassHash) (felt.CasmClassHash, error)
	ClassTrie() (*trie.Trie, error)
	ContractTrie() (*trie.Trie, error)
	ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error)
}

type StateSnapshotReader struct {
	txn db.IndexedBatch
}

func NewStateSnapshotReader(txn db.IndexedBatch) *StateSnapshotReader {
	return &StateSnapshotReader{
		txn: txn,
	}
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

func (s *StateSnapshotReader) ClassTrie() (*trie.Trie, error) {
	// We don't need to call the closer function here because we are only reading the trie
	tr, _, err := classesTrie(s.txn)
	return tr, err
}

func (s *StateSnapshotReader) ContractTrie() (*trie.Trie, error) {
	tr, _, err := contractTrie(s.txn)
	return tr, err
}

func (s *StateSnapshotReader) ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error) {
	return storage(addr, s.txn)
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

// todo(rdr): return `StateDiff` by value
func (s *StateSnapshotReader) GetReverseStateDiff(
	blockNumber uint64,
	diff *StateDiff,
) (*StateDiff, error) {
	reversed := *diff

	reversed.StorageDiffs = make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs))
	for addr, storageDiffs := range diff.StorageDiffs {
		reversedDiffs := make(map[felt.Felt]*felt.Felt, len(storageDiffs))
		for key := range storageDiffs {
			value := felt.Zero
			if blockNumber > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNumber-1)
				if err != nil {
					return nil, err
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
				return nil, err
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
				return nil, err
			}
		}
		reversed.ReplacedClasses[addr] = &classHash
	}

	return &reversed, nil
}

// Root returns the state commitment.
func (s *StateSnapshotReader) Commitment() (felt.Felt, error) {
	var storageRoot, classesRoot felt.Felt

	sStorage, closer, err := contractTrie(s.txn)
	if err != nil {
		return felt.Felt{}, err
	}

	if storageRoot, err = sStorage.Hash(); err != nil {
		return felt.Felt{}, err
	}

	if err = closer(); err != nil {
		return felt.Felt{}, err
	}

	classes, closer, err := classesTrie(s.txn)
	if err != nil {
		return felt.Felt{}, err
	}

	if classesRoot, err = classes.Hash(); err != nil {
		return felt.Felt{}, err
	}

	if err = closer(); err != nil {
		return felt.Felt{}, err
	}

	if classesRoot.IsZero() {
		return storageRoot, nil
	}

	root := crypto.PoseidonArray(stateVersion, &storageRoot, &classesRoot)
	return root, nil
}

func (s *StateSnapshotReader) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	classTrie, closer, err := classesTrie(s.txn)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	defer func() {
		if closeErr := closer(); closeErr != nil {
			_ = closeErr
		}
	}()

	casmHash, err := classTrie.Get((*felt.Felt)(classHash))
	if err != nil {
		return felt.CasmClassHash{}, err
	}

	if casmHash.IsZero() {
		return felt.CasmClassHash{}, errors.New("casm hash not found")
	}

	return felt.CasmClassHash(casmHash), nil
}

func (s *StateSnapshotReader) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	return GetCasmClassHashV2(s.txn, classHash)
}
