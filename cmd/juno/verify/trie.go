package verify

import (
	"bytes"
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
)

const (
	starknetTrieHeight = 251
)

type TrieType string

const (
	ContractTrieType        TrieType = "contract"
	ClassTrieType           TrieType = "class"
	ContractStorageTrieType TrieType = "contract-storage"
)

type TrieConfig struct {
	Tries []TrieType
}

type TrieVerifier struct {
	database db.KeyValueStore
	logger   utils.SimpleLogger
}

func NewTrieVerifier(database db.KeyValueStore, logger utils.SimpleLogger) *TrieVerifier {
	return &TrieVerifier{
		database: database,
		logger:   logger,
	}
}

func (v *TrieVerifier) Name() string {
	return "trie"
}

func (v *TrieVerifier) DefaultConfig() Config {
	return &TrieConfig{
		Tries: nil,
	}
}

func (v *TrieVerifier) Run(ctx context.Context, cfg Config) error {
	trieCfg, ok := cfg.(*TrieConfig)
	if !ok {
		return fmt.Errorf("invalid config type for trie verifier: expected *TrieConfig")
	}

	typesToVerify := trieCfg.Tries
	if len(typesToVerify) == 0 {
		typesToVerify = []TrieType{ContractTrieType, ClassTrieType, ContractStorageTrieType}
	}

	typeSet := make(map[TrieType]bool)
	for _, t := range typesToVerify {
		typeSet[t] = true
	}

	var allErrors []error

	if typeSet[ContractTrieType] {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stateTrieInfo := TrieInfo{
			Name:     "ContractTrie",
			Bucket:   db.StateTrie,
			HashFunc: trie.NewTriePedersen,
			ReaderFunc: func(r db.KeyValueReader, prefix []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: starknetTrieHeight,
		}

		v.logger.Infow("=== Scanning ContractTrie ===")
		if err := v.scanTrie(v.database, stateTrieInfo); err != nil {
			v.logger.Errorw("Error scanning ContractTrie", "error", err)
			allErrors = append(allErrors, fmt.Errorf("ContractTrie: %w", err))
		}
	}

	if typeSet[ClassTrieType] {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		classTrieInfo := TrieInfo{
			Name:     "ClassesTrie",
			Bucket:   db.ClassesTrie,
			HashFunc: trie.NewTriePoseidon,
			ReaderFunc: func(r db.KeyValueReader, prefix []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPoseidon(r, prefix, height)
			},
			Height: starknetTrieHeight,
		}

		v.logger.Infow("=== Scanning ClassTrie ===")
		if err := v.scanTrie(v.database, classTrieInfo); err != nil {
			v.logger.Errorw("Error scanning ClassTrie", "error", err)
			allErrors = append(allErrors, fmt.Errorf("ClassTrie: %w", err))
		}
	}

	if typeSet[ContractStorageTrieType] {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		contractAddresses := v.collectContractAddresses()
		if len(contractAddresses) > 0 {
			v.logger.Infow("=== Scanning Contract Storage Tries ===")
			v.logger.Infow("Found contracts to scan", "count", len(contractAddresses))

			contractErrors := v.scanContractStorageTries(v.database, contractAddresses)
			if len(contractErrors) > 0 {
				v.logger.Errorw("Errors in contracts", "errorCount", len(contractErrors), "totalCount", len(contractAddresses))
				allErrors = append(allErrors, fmt.Errorf("contract storage tries: %d errors", len(contractErrors)))
				for _, err := range contractErrors {
					v.logger.Errorw("Contract error", "error", err)
				}
			}
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("trie verification completed with errors: %v", allErrors)
	}

	v.logger.Infow("=== Trie verification completed successfully ===")
	return nil
}

type TrieInfo struct {
	Name       string
	Bucket     db.Bucket
	HashFunc   trie.NewTrieFunc
	ReaderFunc func(db.KeyValueReader, []byte, uint8) (trie.TrieReader, error)
	Height     uint8
}

func (v *TrieVerifier) scanTrie(database db.KeyValueStore, trieInfo TrieInfo) error {
	prefix := trieInfo.Bucket.Key()
	return v.scanTrieWithPrefix(database, trieInfo, prefix)
}

func (v *TrieVerifier) collectContractAddresses() []felt.Felt {
	contractAddresses := make([]felt.Felt, 0)
	stateTriePrefix := db.StateTrie.Key()

	err := v.database.View(func(snap db.Snapshot) error {
		it, err := snap.NewIterator(stateTriePrefix, true)
		if err != nil {
			return err
		}
		defer it.Close()

		for it.First(); it.Valid(); it.Next() {
			keyBytes := it.Key()
			if bytes.Equal(keyBytes, stateTriePrefix) {
				continue
			}

			if !bytes.HasPrefix(keyBytes, stateTriePrefix) {
				continue
			}
			nodeKeyBytes := keyBytes[len(stateTriePrefix):]

			var nodeKey trie.BitArray
			if err := nodeKey.UnmarshalBinary(nodeKeyBytes); err != nil {
				continue
			}

			if nodeKey.Len() == starknetTrieHeight {
				contractAddr := nodeKey.Felt()
				contractAddresses = append(contractAddresses, contractAddr)
			}
		}
		return nil
	})
	if err != nil {
		return nil
	}

	return contractAddresses
}

func (v *TrieVerifier) scanContractStorageTries(database db.KeyValueStore, contractAddresses []felt.Felt) []error {
	var allErrors []error

	for i, contractAddr := range contractAddresses {
		v.logger.Infow("Scanning contract", "current", i+1, "total", len(contractAddresses))

		addrBytes := contractAddr.Marshal()
		prefix := db.ContractStorage.Key(addrBytes)

		trieInfo := TrieInfo{
			Name:     fmt.Sprintf("ContractStorage[%s]", contractAddr.String()),
			Bucket:   db.ContractStorage,
			HashFunc: trie.NewTriePedersen,
			ReaderFunc: func(r db.KeyValueReader, _ []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: starknetTrieHeight,
		}

		err := v.scanTrieWithPrefix(database, trieInfo, prefix)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("contract %s: %w", contractAddr.String(), err))
		}
	}

	v.logger.Infow("Scanned all contracts", "count", len(contractAddresses))

	return allErrors
}

func (v *TrieVerifier) scanTrieWithPrefix(database db.KeyValueStore, trieInfo TrieInfo, prefix []byte) error {
	var storedRootHash felt.Felt
	var hasRootKey bool
	err := database.View(func(snap db.Snapshot) error {
		reader, err := trieInfo.ReaderFunc(snap, prefix, trieInfo.Height)
		if err != nil {
			return err
		}
		if reader.RootKey() == nil {
			storedRootHash = felt.Zero
			hasRootKey = false
			return nil
		}
		hasRootKey = true
		storedRootHash, err = reader.Hash()
		return err
	})
	if err != nil {
		if trieInfo.Bucket == db.ContractStorage {
			return nil
		}
		return fmt.Errorf("failed to get stored root hash: %w", err)
	}

	isContractStorage := trieInfo.Bucket == db.ContractStorage
	if !isContractStorage {
		v.logger.Infow("Stored root hash", "hash", storedRootHash.String())
		v.logger.Infow("Scanning nodes...")
	}

	leaves := make(map[felt.Felt]felt.Felt)
	totalNodes := 0
	leafCount := 0

	err = database.View(func(snap db.Snapshot) error {
		it, err := snap.NewIterator(prefix, true)
		if err != nil {
			return err
		}
		defer it.Close()

		for it.First(); it.Valid(); it.Next() {
			keyBytes := it.Key()
			if bytes.Equal(keyBytes, prefix) {
				continue
			}

			if !bytes.HasPrefix(keyBytes, prefix) {
				continue
			}
			nodeKeyBytes := keyBytes[len(prefix):]

			var nodeKey trie.BitArray
			if err := nodeKey.UnmarshalBinary(nodeKeyBytes); err != nil {
				continue
			}

			totalNodes++

			valBytes, err := it.Value()
			if err != nil {
				return fmt.Errorf("failed to read value: %w", err)
			}

			var node trie.Node
			if err := node.UnmarshalBinary(valBytes); err != nil {
				return fmt.Errorf("failed to unmarshal node at key %s: %w", nodeKey.String(), err)
			}

			isLeaf := nodeKey.Len() == trieInfo.Height

			if isLeaf && node.Value != nil {
				leafCount++
				keyFelt := nodeKey.Felt()
				leaves[keyFelt] = *node.Value
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to iterate nodes: %w", err)
	}

	if len(leaves) == 0 {
		v.logger.Infow("No leaves found, calculating root from empty trie")
		if isContractStorage && !hasRootKey {
			return nil
		}
		if isContractStorage && hasRootKey {
			return nil
		}
	} else {
		v.logger.Infow("Rebuilding trie from leaves", "leafCount", len(leaves))
	}

	calculatedRoot, err := v.rebuildTrieFromLeaves(leaves, trieInfo)
	if err != nil {
		return fmt.Errorf("failed to rebuild trie: %w", err)
	}

	if calculatedRoot.Equal(&storedRootHash) {
		v.logger.Infow("Root hashes match - no corruption detected",
			"calculated", calculatedRoot.String(),
			"stored", storedRootHash.String())
		return nil
	}

	v.logger.Errorw("ROOT MISMATCH DETECTED!",
		"calculated", calculatedRoot.String(),
		"stored", storedRootHash.String())
	return fmt.Errorf("root hash mismatch for %s, expected %s, got %s", trieInfo.Name, calculatedRoot.String(), storedRootHash.String())
}

func (v *TrieVerifier) rebuildTrieFromLeaves(leaves map[felt.Felt]felt.Felt, trieInfo TrieInfo) (felt.Felt, error) {
	memoryDB := memory.New()
	txn := memoryDB.NewIndexedBatch()
	defer func() {
		_ = memoryDB.Close()
	}()

	t, err := trieInfo.HashFunc(txn, nil, trieInfo.Height)
	if err != nil {
		return felt.Zero, fmt.Errorf("failed to create in-memory trie: %w", err)
	}

	inserted := 0

	for key, value := range leaves {
		keyCopy := key
		valueCopy := value
		if _, err := t.Put(&keyCopy, &valueCopy); err != nil {
			return felt.Zero, fmt.Errorf("failed to insert leaf %s: %w", key.String(), err)
		}
		inserted++
	}

	v.logger.Infow("Inserted all leaves", "count", inserted)
	v.logger.Infow("Calculating root hash...")

	rootHash, err := t.Hash()
	if err != nil {
		return felt.Zero, fmt.Errorf("failed to calculate root hash: %w", err)
	}

	return rootHash, nil
}
