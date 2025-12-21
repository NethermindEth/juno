package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

const (
	starknetTrieHeight  = 251
	concurrencyMaxDepth = 8
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

type TrieInfo struct {
	Name       string
	prefix     []byte
	HashFunc   trie.NewTrieFunc
	HashFn     crypto.HashFn
	ReaderFunc func(db.KeyValueReader, []byte, uint8) (trie.TrieReader, error)
	Height     uint8
}

func (v *TrieVerifier) Run(ctx context.Context, cfg Config) error {
	startTime := time.Now()
	defer func() {
		elapsed := time.Since(startTime)
		v.logger.Infow("=== Trie verification finished ===", "total_elapsed", elapsed.Round(time.Second))
	}()

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

	if typeSet[ContractTrieType] {
		stateTrieInfo := TrieInfo{
			Name:       "ContractsTrie",
			prefix:     db.StateTrie.Key(),
			HashFunc:   trie.NewTriePedersen,
			HashFn:     crypto.Pedersen,
			ReaderFunc: trie.NewTrieReaderPedersen,
			Height:     starknetTrieHeight,
		}
		if err := v.verifyTrieWithLogging(ctx, stateTrieInfo); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}

	if typeSet[ClassTrieType] {
		classTrieInfo := TrieInfo{
			Name:       "ClassesTrie",
			prefix:     db.ClassesTrie.Key(),
			HashFunc:   trie.NewTriePoseidon,
			HashFn:     crypto.Poseidon,
			ReaderFunc: trie.NewTrieReaderPoseidon,
			Height:     starknetTrieHeight,
		}
		if err := v.verifyTrieWithLogging(ctx, classTrieInfo); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}

	if typeSet[ContractStorageTrieType] {
		if err := v.verifyContractStorageTries(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}

	v.logger.Infow("=== Trie verification completed successfully ===")
	return nil
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

func (v *TrieVerifier) verifyTrieWithLogging(ctx context.Context, trieInfo TrieInfo) error {
	v.logger.Infow(fmt.Sprintf("=== Starting %s verification ===", trieInfo.Name))
	err := v.verifyTrie(ctx, trieInfo)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			v.logger.Infow("Verification stopped", "trie", trieInfo.Name)
			return err
		}
		v.logger.Errorw(fmt.Sprintf("%s verification failed", trieInfo.Name), "error", err)
		return fmt.Errorf("%s verification failed: %w", trieInfo.Name, err)
	}
	v.logger.Infow(fmt.Sprintf("%s verification completed successfully", trieInfo.Name))
	return nil
}

func (v *TrieVerifier) verifyTrie(ctx context.Context, trieInfo TrieInfo) error {
	expectedRoot := felt.Zero
	err := v.database.View(func(snap db.Snapshot) error {
		reader, err := trieInfo.ReaderFunc(snap, trieInfo.prefix, trieInfo.Height)
		if err != nil {
			return err
		}
		if reader.RootKey() == nil {
			expectedRoot = felt.Zero
			return nil
		}
		expectedRoot, err = reader.Hash()
		return err
	})
	if err != nil {
		v.logger.Errorw("Failed to get stored root hash", "trie", trieInfo.Name, "error", err)
		return fmt.Errorf("failed to get stored root hash for %s: %w", trieInfo.Name, err)
	}

	if expectedRoot.IsZero() {
		v.logger.Infow("Trie is empty (zero root)", "trie", trieInfo.Name)
		return nil
	}

	v.logger.Infow("Starting verification",
		"trie",
		trieInfo.Name,
		"expectedRoot",
		expectedRoot.String(),
	)
	storageReader := trie.NewReadStorage(v.database, trieInfo.prefix)

	err = verifyTrie(ctx, storageReader, starknetTrieHeight, trieInfo.HashFn, &expectedRoot)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		v.logger.Errorw("Trie verification failed", "trie", trieInfo.Name, "error", err)
		return err
	}

	v.logger.Infow("Trie verification successful",
		"trie", trieInfo.Name, "root",
		expectedRoot.String(),
	)
	return nil
}

func (v *TrieVerifier) verifyContractStorageTries(ctx context.Context) error {
	v.logger.Infow("=== Starting Contract Storage Tries verification ===")
	contractAddresses := v.collectContractAddresses()

	if len(contractAddresses) == 0 {
		v.logger.Infow("No contract addresses found, skipping contract storage verification")
		return nil
	}

	v.logger.Infow("Found contracts to verify", "count", len(contractAddresses))

	for i, contractAddress := range contractAddresses {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		addrBytes := contractAddress.Marshal()
		prefix := db.ContractStorage.Key(addrBytes)
		trieInfo := TrieInfo{
			Name:     fmt.Sprintf("ContractStorage[%s]", contractAddress.String()),
			prefix:   db.ContractStorage.Key(addrBytes),
			HashFunc: trie.NewTriePedersen,
			HashFn:   crypto.Pedersen,
			ReaderFunc: func(r db.KeyValueReader, _ []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: starknetTrieHeight,
		}

		v.logger.Infow(
			"Verifying contract storage",
			"contract",
			contractAddress.String(),
			"progress",
			fmt.Sprintf("%d/%d", i+1, len(contractAddresses)),
		)

		err := v.verifyTrie(ctx, trieInfo)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			v.logger.Errorw(
				"Contract storage verification failed",
				"contract",
				contractAddress.String(),
				"error",
				err,
			)
			return fmt.Errorf(
				"contract storage verification failed for %s: %w",
				contractAddress.String(), err,
			)
		}
	}

	v.logger.Infow("All contract storage tries verified successfully", "count", len(contractAddresses))
	return nil
}

func verifyTrie(
	ctx context.Context,
	reader *trie.ReadStorage,
	height uint8,
	hashFn crypto.HashFn,
	expectedRoot *felt.Felt,
) error {
	rootKey, err := reader.RootKey()
	if err != nil {
		return fmt.Errorf("failed to get root key: %w", err)
	}

	if rootKey == nil {
		return nil
	}

	startTime := time.Now()
	rootHash, err := verifyNode(ctx, reader, rootKey, nil, height, hashFn)
	if err != nil {
		return fmt.Errorf("node verification failed: %w", err)
	}

	elapsed := time.Since(startTime)

	if rootHash.Cmp(expectedRoot) != 0 {
		return fmt.Errorf(
			"root hash mismatch: expected %s, got %s (verification took %v)",
			expectedRoot, rootHash, elapsed.Round(time.Second),
		)
	}

	return nil
}

func verifyNode(
	ctx context.Context,
	reader *trie.ReadStorage,
	key *trie.BitArray,
	parentKey *trie.BitArray,
	height uint8,
	hashFn crypto.HashFn,
) (*felt.Felt, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("verification cancelled: %w", ctx.Err())
	default:
	}

	node, err := reader.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get node at key %s: %w", key.String(), err)
	}

	if key.Len() == height {
		p := path(key, parentKey)
		h := node.Hash(&p, hashFn)
		return &h, nil
	}

	useConcurrency := key.Len() <= concurrencyMaxDepth
	var leftHash, rightHash *felt.Felt
	var leftErr, rightErr error

	if useConcurrency {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if node.Left.IsEmpty() {
				zero := felt.Zero
				leftHash = &zero
				return
			}
			h, err := verifyNode(ctx, reader, node.Left, key, height, hashFn)
			leftHash = h
			leftErr = err
		}()

		if node.Right.IsEmpty() {
			zero := felt.Zero
			rightHash = &zero
		} else {
			h, err := verifyNode(ctx, reader, node.Right, key, height, hashFn)
			rightHash = h
			rightErr = err
		}

		wg.Wait()

		if leftErr != nil {
			return nil, leftErr
		}
		if rightErr != nil {
			return nil, rightErr
		}
	} else {
		if node.Left.IsEmpty() {
			zero := felt.Zero
			leftHash = &zero
		} else {
			h, err := verifyNode(ctx, reader, node.Left, key, height, hashFn)
			if err != nil {
				return nil, err
			}
			leftHash = h
		}

		if node.Right.IsEmpty() {
			zero := felt.Zero
			rightHash = &zero
		} else {
			h, err := verifyNode(ctx, reader, node.Right, key, height, hashFn)
			if err != nil {
				return nil, err
			}
			rightHash = h
		}
	}

	recomputed := hashFn(leftHash, rightHash)
	if recomputed.Cmp(node.Value) != 0 {
		return nil, fmt.Errorf(
			"node corruption detected at key %s: stored hash=%s, recomputed hash=%s",
			key.String(), node.Value.String(), recomputed.String(),
		)
	}

	tmp := *node
	tmp.Value = &recomputed

	p := path(key, parentKey)
	h := tmp.Hash(&p, hashFn)
	return &h, nil
}

func path(key, parentKey *trie.BitArray) trie.BitArray {
	if parentKey == nil {
		return key.Copy()
	}

	var pathKey trie.BitArray
	pathKey.LSBs(key, parentKey.Len()+1)
	return pathKey
}
