package core

import (
	"bytes"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
)

type TrieInfo struct {
	Name       string
	Bucket     db.Bucket
	HashFunc   trie.NewTrieFunc
	ReaderFunc func(db.KeyValueReader, []byte, uint8) (trie.TrieReader, error)
	Height     uint8
}

func RecoveryScan(database db.KeyValueStore) error {
	var allErrors []error

	tries := []TrieInfo{
		{
			Name:     "ClassesTrie",
			Bucket:   db.ClassesTrie,
			HashFunc: trie.NewTriePoseidon,
			ReaderFunc: func(r db.KeyValueReader, prefix []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPoseidon(r, prefix, height)
			},
			Height: globalTrieHeight,
		},
		{
			Name:     "StateTrie",
			Bucket:   db.StateTrie,
			HashFunc: trie.NewTriePedersen,
			ReaderFunc: func(r db.KeyValueReader, prefix []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: globalTrieHeight,
		},
	}

	for _, trieInfo := range tries {
		fmt.Printf("\n=== Scanning %s ===\n", trieInfo.Name)
		if err := scanTrie(database, trieInfo); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning %s: %v\n", trieInfo.Name, err)
			allErrors = append(allErrors, fmt.Errorf("%s: %w", trieInfo.Name, err))
		}
	}

	contractAddresses := collectContractAddresses(database)
	if len(contractAddresses) > 0 {
		fmt.Printf("\n=== Scanning Contract Storage Tries ===\n")
		fmt.Printf("  Found %d contracts to scan\n", len(contractAddresses))

		contractErrors := scanContractStorageTries(database, contractAddresses)
		if len(contractErrors) > 0 {
			fmt.Fprintf(os.Stderr, "  Errors in %d/%d contracts\n", len(contractErrors), len(contractAddresses))
			allErrors = append(allErrors, fmt.Errorf("contract storage tries: %d errors", len(contractErrors)))
			for _, err := range contractErrors {
				fmt.Fprintf(os.Stderr, "    %v\n", err)
			}
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("recovery scan completed with errors: %v", allErrors)
	}

	fmt.Println("\n=== Recovery scan completed successfully ===")
	return nil
}

func scanTrie(database db.KeyValueStore, trieInfo TrieInfo) error {
	prefix := trieInfo.Bucket.Key()
	return scanTrieWithPrefix(database, trieInfo, prefix)
}

func collectContractAddresses(database db.KeyValueStore) []felt.Felt {
	contractAddresses := make([]felt.Felt, 0)
	stateTriePrefix := db.StateTrie.Key()

	err := database.View(func(snap db.Snapshot) error {
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

			if nodeKey.Len() == globalTrieHeight {
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

func scanContractStorageTries(database db.KeyValueStore, contractAddresses []felt.Felt) []error {
	var allErrors []error

	for i, contractAddr := range contractAddresses {
		fmt.Printf("\n=== Scanning %s ===\n", contractAddr.String())
		fmt.Printf("  Scanning contract %d/%d...\n", i+1, len(contractAddresses))

		addrBytes := contractAddr.Marshal()
		prefix := db.ContractStorage.Key(addrBytes)

		trieInfo := TrieInfo{
			Name:     fmt.Sprintf("ContractStorage[%s]", contractAddr.String()),
			Bucket:   db.ContractStorage,
			HashFunc: trie.NewTriePedersen,
			ReaderFunc: func(r db.KeyValueReader, _ []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: ContractStorageTrieHeight,
		}

		err := scanTrieWithPrefix(database, trieInfo, prefix)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("contract %s: %w", contractAddr.String(), err))
		}
	}

	fmt.Printf("  Scanned all %d contracts\n", len(contractAddresses))

	return allErrors
}

func scanTrieWithPrefix(database db.KeyValueStore, trieInfo TrieInfo, prefix []byte) error {
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
		fmt.Printf("  Stored root hash: %s\n", storedRootHash.String())
		fmt.Printf("  Scanning nodes...\n")
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
		fmt.Printf("  No leaves found, calculating root from empty trie\n")
		if isContractStorage && !hasRootKey {
			return nil
		}
		if isContractStorage && hasRootKey {
			return nil
		}
	} else {
		fmt.Printf("  Rebuilding trie from %d leaves...\n", len(leaves))
	}

	calculatedRoot, err := rebuildTrieFromLeaves(leaves, trieInfo)
	if err != nil {
		return fmt.Errorf("failed to rebuild trie: %w", err)
	}

	if calculatedRoot.Equal(&storedRootHash) {
		fmt.Printf("    Root hashes match - no corruption detected\n")
		fmt.Printf("    Calculated: %s\n", calculatedRoot.String())
		fmt.Printf("    Stored:     %s\n", storedRootHash.String())
		return nil
	}

	fmt.Printf("    ROOT MISMATCH DETECTED!\n")
	fmt.Printf("    Calculated: %s\n", calculatedRoot.String())
	fmt.Printf("    Stored:     %s\n", storedRootHash.String())
	return fmt.Errorf("root hash mismatch for %s", trieInfo.Name)
}

func rebuildTrieFromLeaves(leaves map[felt.Felt]felt.Felt, trieInfo TrieInfo) (felt.Felt, error) {
	memoryDB := memory.New()
	txn := memoryDB.NewIndexedBatch()
	defer func() {
		_ = memoryDB.Close()
	}()

	t, err := trieInfo.HashFunc(txn, nil, trieInfo.Height)
	if err != nil {
		return felt.Zero, fmt.Errorf("failed to create in-memory trie: %w", err)
	}

	totalLeaves := len(leaves)
	const rebuildProgressInterval = 10000
	inserted := 0

	for key, value := range leaves {
		keyCopy := key
		valueCopy := value
		if _, err := t.Put(&keyCopy, &valueCopy); err != nil {
			return felt.Zero, fmt.Errorf("failed to insert leaf %s: %w", key.String(), err)
		}
		inserted++
	}

	if totalLeaves > 0 && inserted%rebuildProgressInterval != 0 {
		fmt.Printf("    Inserted all %d leaves\n", inserted)
	}

	fmt.Printf("    Calculating root hash...\n")

	rootHash, err := t.Hash()
	if err != nil {
		return felt.Zero, fmt.Errorf("failed to calculate root hash: %w", err)
	}

	return rootHash, nil
}
