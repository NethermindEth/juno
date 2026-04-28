package state

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

// StateReader is a read-only view of the state at a given root.
type StateReader struct {
	initRoot     felt.Felt
	db           *StateDB
	contractTrie *trie2.Trie
	classTrie    *trie2.Trie
}

// NewStateReader creates a read-only view of the state at the given root.
// Should be used for read operations that don't require state mutations.
func NewStateReader(stateRoot *felt.Felt, db *StateDB) (*StateReader, error) {
	contractTrie, err := db.ContractTrie(stateRoot)
	if err != nil {
		return nil, err
	}

	classTrie, err := db.ClassTrie(stateRoot)
	if err != nil {
		return nil, err
	}

	return &StateReader{
		initRoot:     *stateRoot,
		db:           db,
		contractTrie: contractTrie,
		classTrie:    classTrie,
	}, nil
}

func (s *StateReader) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	contract, err := GetContract(s.db.disk, addr)
	if err != nil {
		return felt.Felt{}, err
	}
	return contract.ClassHash, nil
}

func (s *StateReader) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	contract, err := GetContract(s.db.disk, addr)
	if err != nil {
		return felt.Felt{}, err
	}
	return contract.Nonce, nil
}

// ContractStorage reads a storage slot directly from the trie at this reader's
// root. Missing slots read as felt.Zero.
func (s *StateReader) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	path := trieutils.FeltToPath(key, ContractStorageTrieHeight)
	v, err := trieutils.GetNodeByPath(
		s.db.disk,
		db.ContractTrieStorage,
		(*felt.Address)(addr),
		&path,
		true,
	)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return felt.Zero, nil
		}
		return felt.Zero, err
	}

	return felt.FromBytes[felt.Felt](v), nil
}

// ContractStorageLastUpdatedBlock returns the most recent block number at which a given storage
// slot key of a given contract was last updated.
func (s *StateReader) ContractStorageLastUpdatedBlock(
	addr *felt.Address,
	key *felt.Felt,
) (uint64, error) {
	prefix := db.ContractStorageHistoryKey((*felt.Felt)(addr), key)
	return s.lastUpdatedBlockNumber(prefix, math.MaxUint64)
}

func (s *StateReader) ContractDeployedAt(addr *felt.Felt, blockNum uint64) (bool, error) {
	contract, err := GetContract(s.db.disk, addr)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}

	return contract.DeployedHeight <= blockNum, nil
}

func (s *StateReader) Class(classHash *felt.Felt) (*core.DeclaredClassDefinition, error) {
	return GetClass(s.db.disk, classHash)
}

func (s *StateReader) ClassTrie() (core.TrieReader, error) {
	return s.classTrie, nil
}

func (s *StateReader) ContractTrie() (core.TrieReader, error) {
	return s.contractTrie, nil
}

func (s *StateReader) ContractStorageTrie(addr *felt.Felt) (core.TrieReader, error) {
	return s.db.ContractStorageTrie(&s.initRoot, addr)
}

func (s *StateReader) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	metadata, err := core.GetClassCasmHashMetadata(s.db.disk, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHash(), nil
}

func (s *StateReader) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	metadata, err := core.GetClassCasmHashMetadata(s.db.disk, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHashV2(), nil
}

func (s *StateReader) Commitment(protocolVersion string) (felt.Felt, error) {
	contractRoot, err := s.contractTrie.Hash()
	if err != nil {
		return felt.Felt{}, err
	}
	classRoot, err := s.classTrie.Hash()
	if err != nil {
		return felt.Felt{}, err
	}
	return stateCommitment(&contractRoot, &classRoot, protocolVersion), nil
}

func (s *StateReader) GetReverseStateDiff(
	blockNum uint64,
	diff *core.StateDiff,
) (core.StateDiff, error) {
	reverse := core.StateDiff{
		StorageDiffs:    make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(diff.StorageDiffs)),
		Nonces:          make(map[felt.Felt]*felt.Felt, len(diff.Nonces)),
		ReplacedClasses: make(map[felt.Felt]*felt.Felt, len(diff.ReplacedClasses)),
	}

	for addr, stDiffs := range diff.StorageDiffs {
		reverse.StorageDiffs[addr] = make(map[felt.Felt]*felt.Felt, len(stDiffs))
		for key := range stDiffs {
			value := felt.Zero
			if blockNum > 0 {
				oldValue, err := s.ContractStorageAt(&addr, &key, blockNum-1)
				if err != nil {
					return core.StateDiff{}, err
				}
				value = oldValue
			}
			reverse.StorageDiffs[addr][key] = &value
		}
	}

	for addr := range diff.Nonces {
		oldNonce := felt.Zero
		if blockNum > 0 {
			var err error
			oldNonce, err = s.ContractNonceAt(&addr, blockNum-1)
			if err != nil {
				return core.StateDiff{}, err
			}
		}
		reverse.Nonces[addr] = &oldNonce
	}

	for addr := range diff.ReplacedClasses {
		oldCh := felt.Zero
		if blockNum > 0 {
			var err error
			oldCh, err = s.ContractClassHashAt(&addr, blockNum-1)
			if err != nil {
				return core.StateDiff{}, err
			}
		}
		reverse.ReplacedClasses[addr] = &oldCh
	}

	return reverse, nil
}

// Returns the storage value of a contract at a given storage key at a given block number.
func (s *StateReader) ContractStorageAt(addr, key *felt.Felt, blockNum uint64) (felt.Felt, error) {
	prefix := db.ContractStorageHistoryKey(addr, key)
	return s.getHistoricalValue(prefix, blockNum)
}

// Returns the block number at which a given storage slot key of a given contract was
// last updated, up to and including the given block number.
func (s *StateReader) ContractStorageLastUpdatedAt(
	addr *felt.Address,
	key *felt.Felt,
	blockNum uint64,
) (uint64, error) {
	prefix := db.ContractStorageHistoryKey((*felt.Felt)(addr), key)
	return s.lastUpdatedBlockNumber(prefix, blockNum)
}

// Returns the nonce of a contract at a given block number.
func (s *StateReader) ContractNonceAt(addr *felt.Felt, blockNum uint64) (felt.Felt, error) {
	prefix := db.ContractNonceHistoryKey(addr)
	return s.getHistoricalValue(prefix, blockNum)
}

// Returns the class hash of a contract at a given block number.
func (s *StateReader) ContractClassHashAt(addr *felt.Felt, blockNum uint64) (felt.Felt, error) {
	prefix := db.ContractClassHashHistoryKey(addr)
	return s.getHistoricalValue(prefix, blockNum)
}

func (s *StateReader) CompiledClassHashAt(
	classHash *felt.SierraClassHash,
	blockNumber uint64,
) (felt.CasmClassHash, error) {
	metadata, err := core.GetClassCasmHashMetadata(s.db.disk, classHash)
	if err != nil {
		return felt.CasmClassHash{}, err
	}
	return metadata.CasmHashAt(blockNumber)
}

func (s *StateReader) getHistoricalValue(prefix []byte, blockNum uint64) (felt.Felt, error) {
	var ret felt.Felt

	err := s.valueAt(prefix, blockNum, func(val []byte) error {
		ret.SetBytes(val)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNoHistoryValue) {
			return felt.Zero, nil
		}
		return felt.Zero, err
	}

	return ret, nil
}

// Returns the value at the given block number.
// Seeks to the entry for blockNum if it exists; otherwise steps back to the
// last recorded block strictly less than blockNum. Returns ErrNoHistoryValue
// if no entry exists at or before blockNum.
//
// Each history entry stores the post-update value at the block it was written.
// Example — a nonce updated at blocks 10 and 25:
//
//	stored: (prefix, 10) -> 7   (prefix, 25) -> 42
//	query @ 5  -> no entry <= 5               -> ErrNoHistoryValue (-> Zero)
//	query @ 10 -> exact hit on (prefix, 10)   -> 7
//	query @ 20 -> Seek lands on 25, Prev -> 10 -> 7
//	query @ 30 -> Seek fails, Prev -> 25       -> 42
func (s *StateReader) valueAt(prefix []byte, blockNum uint64, cb func(val []byte) error) error {
	it, err := s.db.disk.NewIterator(prefix, true)
	if err != nil {
		return err
	}
	defer it.Close()

	seekKey := binary.BigEndian.AppendUint64(prefix, blockNum)

	if !it.Seek(seekKey) || binary.BigEndian.Uint64(it.Key()[len(prefix):]) != blockNum {
		if !it.Prev() {
			return ErrNoHistoryValue
		}
	}

	val, err := it.Value()
	if err != nil {
		return err
	}
	return cb(val)
}

// lastUpdatedBlockNumber finds the most recent block number (up to upToBlock) recorded in
// a history bucket for the given key prefix. All history buckets ([ContractStorageHistory],
// [ContractNonceHistory], and [ContractClassHashHistory]) share the same key layout:
// prefix + uint64 block number in big-endian.
func (s *StateReader) lastUpdatedBlockNumber(
	historyKeyPrefix []byte,
	upToBlock uint64,
) (uint64, error) {
	it, err := s.db.disk.NewIterator(historyKeyPrefix, true)
	if err != nil {
		return 0, err
	}
	defer it.Close()

	seekKey := binary.BigEndian.AppendUint64(historyKeyPrefix, upToBlock)

	if it.Seek(seekKey) {
		seekedKey := it.Key()
		seekedBlock := binary.BigEndian.Uint64(seekedKey[len(historyKeyPrefix):])
		if seekedBlock == upToBlock {
			return upToBlock, nil
		}
	}

	if !it.Prev() {
		return 0, nil
	}

	foundKey := it.Key()
	blockNum := binary.BigEndian.Uint64(foundKey[len(historyKeyPrefix):])
	return blockNum, nil
}

// Calculate the commitment of the state.
// Since v0.14.0, the Poseidon hash is always applied even when classRoot is zero.
func stateCommitment(contractRoot, classRoot *felt.Felt, protocolVersion string) felt.Felt {
	if classRoot.IsZero() && contractRoot.IsZero() {
		return felt.Zero
	}

	ver, _ := core.ParseBlockVersion(protocolVersion)
	if classRoot.IsZero() && ver.LessThan(core.Ver0_14_0) {
		return *contractRoot
	}

	return crypto.PoseidonArray(stateVersion0, contractRoot, classRoot)
}
