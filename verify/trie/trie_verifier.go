package trie

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type TrieVerifier struct {
	database        db.KeyValueStore
	logger          utils.StructuredLogger
	tries           []TrieType
	contractAddress *felt.Felt
}

func NewTrieVerifier(
	database db.KeyValueStore,
	logger utils.StructuredLogger,
	tries []TrieType,
	contractAddress *felt.Felt,
) *TrieVerifier {
	if len(tries) == 0 {
		tries = []TrieType{ContractTrie, ClassTrie, ContractStorageTrie}
	}
	return &TrieVerifier{
		database:        database,
		logger:          logger,
		tries:           tries,
		contractAddress: contractAddress,
	}
}

func (v *TrieVerifier) Name() string {
	return "trie"
}

func (v *TrieVerifier) Run(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		elapsed := time.Since(startTime)
		v.logger.Info("=== Trie verification finished ===",
			zap.Duration("total_elapsed", elapsed.Round(time.Second)))
	}()

	typeSet := make(map[TrieType]bool)
	for _, t := range v.tries {
		typeSet[t] = true
	}

	if typeSet[ContractTrie] {
		stateTrieInfo := TrieInfo{
			Name:       "ContractsTrie",
			Prefix:     db.StateTrie.Key(),
			HashFunc:   trie.NewTriePedersen,
			HashFn:     crypto.Pedersen,
			ReaderFunc: trie.NewTrieReaderPedersen,
			Height:     StarknetTrieHeight,
		}
		if err := v.verifyTrieWithLogging(ctx, stateTrieInfo); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}

	if typeSet[ClassTrie] {
		classTrieInfo := TrieInfo{
			Name:       "ClassesTrie",
			Prefix:     db.ClassesTrie.Key(),
			HashFunc:   trie.NewTriePoseidon,
			HashFn:     crypto.Poseidon,
			ReaderFunc: trie.NewTrieReaderPoseidon,
			Height:     StarknetTrieHeight,
		}
		if err := v.verifyTrieWithLogging(ctx, classTrieInfo); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}

	if typeSet[ContractStorageTrie] {
		if err := v.verifyContractStorageTries(ctx, v.contractAddress); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}

	v.logger.Info("=== Trie verification completed successfully ===")
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

			if nodeKey.Len() == StarknetTrieHeight {
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
	err := v.verifyTrie(ctx, trieInfo)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			v.logger.Info("Verification stopped", zap.String("trie", trieInfo.Name))
			return err
		}
		v.logger.Error(fmt.Sprintf("%s verification failed: %v", trieInfo.Name, err))
		return fmt.Errorf("%s verification failed: %w", trieInfo.Name, err)
	}
	return nil
}

func (v *TrieVerifier) verifyTrie(ctx context.Context, trieInfo TrieInfo) error {
	v.logger.Info(fmt.Sprintf("=== Starting %s verification ===", trieInfo.Name))
	expectedRoot := felt.Zero
	err := v.database.View(func(snap db.Snapshot) error {
		reader, err := trieInfo.ReaderFunc(snap, trieInfo.Prefix, trieInfo.Height)
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
		v.logger.Error(fmt.Sprintf("Failed to get stored root hash for %s: %v", trieInfo.Name, err))
		return fmt.Errorf("failed to get stored root hash for %s: %w", trieInfo.Name, err)
	}

	if expectedRoot.IsZero() {
		v.logger.Info("Trie is empty (zero root)", zap.String("trie", trieInfo.Name))
		return nil
	}

	v.logger.Info("Starting verification",
		zap.String("trie", trieInfo.Name),
		zap.String("expectedRoot", expectedRoot.String()))
	storageReader := trie.NewReadStorage(v.database, trieInfo.Prefix)

	err = VerifyTrie(ctx, storageReader, StarknetTrieHeight, trieInfo.HashFn, &expectedRoot)
	if err != nil {
		return err
	}

	v.logger.Info("Trie verification successful",
		zap.String("trie", trieInfo.Name), zap.String("root", expectedRoot.String()))
	return nil
}

func (v *TrieVerifier) verifyContractStorageTries(
	ctx context.Context, filterAddress *felt.Felt,
) error {
	v.logger.Info("=== Starting Contract Storage Tries verification ===")

	var contractAddresses []felt.Felt
	if filterAddress != nil {
		contractAddresses = []felt.Felt{*filterAddress}
		v.logger.Info("Verifying specific contract",
			zap.String("address", filterAddress.String()))
	} else {
		contractAddresses = v.collectContractAddresses()
		if len(contractAddresses) == 0 {
			v.logger.Info("No contract addresses found, skipping contract storage verification")
			return nil
		}
		v.logger.Info("Found contracts to verify",
			zap.Int("count", len(contractAddresses)))
	}

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
			Prefix:   db.ContractStorage.Key(addrBytes),
			HashFunc: trie.NewTriePedersen,
			HashFn:   crypto.Pedersen,
			ReaderFunc: func(r db.KeyValueReader, _ []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: StarknetTrieHeight,
		}

		v.logger.Info("Verifying contract storage",
			zap.String("contract", contractAddress.String()),
			zap.String("progress", fmt.Sprintf("%d/%d", i+1, len(contractAddresses))))

		err := v.verifyTrie(ctx, trieInfo)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			v.logger.Error(fmt.Sprintf("Contract storage verification failed for %s: %v",
				contractAddress.String(), err))
			return fmt.Errorf(
				"contract storage verification failed for %s: %w",
				contractAddress.String(),
				err,
			)
		}
	}

	v.logger.Info("All contract storage tries verified successfully",
		zap.Int("count", len(contractAddresses)))
	return nil
}
