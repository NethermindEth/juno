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
		v.logger.Info("Trie verification finished",
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
			HashFn:     crypto.Pedersen,
			ReaderFunc: trie.NewTrieReaderPedersen,
			Height:     StarknetTrieHeight,
		}
		if err := v.verifyTrie(ctx, stateTrieInfo); err != nil {
			return v.handleResult(err, stateTrieInfo.Name)
		}
	}

	if typeSet[ClassTrie] {
		classTrieInfo := TrieInfo{
			Name:       "ClassesTrie",
			Prefix:     db.ClassesTrie.Key(),
			HashFn:     crypto.Poseidon,
			ReaderFunc: trie.NewTrieReaderPoseidon,
			Height:     StarknetTrieHeight,
		}
		if err := v.verifyTrie(ctx, classTrieInfo); err != nil {
			return v.handleResult(err, classTrieInfo.Name)
		}
	}

	if typeSet[ContractStorageTrie] {
		if err := v.verifyContractStorageTries(ctx, v.contractAddress); err != nil {
			return v.handleResult(err, "ContractStorageTries")
		}
	}

	v.logger.Info("Trie verification completed successfully")
	return nil
}

func (v *TrieVerifier) handleResult(err error, trieName string) error {
	if errors.Is(err, context.Canceled) {
		v.logger.Info("Verification stopped", zap.String("trie", trieName))
		return nil
	}
	if errors.Is(err, ErrCorruptionDetected) {
		v.logger.Info("Corruption detected",
			zap.String("trie", trieName),
			zap.String("details", err.Error()))
		return err
	}
	v.logger.Error("Verification error",
		zap.String("trie", trieName),
		zap.Error(err))
	return err
}

func (v *TrieVerifier) collectContractAddresses() ([]felt.Felt, error) {
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
		v.logger.Error("Failed to collect contract addresses", zap.Error(err))
		return nil, fmt.Errorf("failed to collect contract addresses: %w", err)
	}

	return contractAddresses, nil
}

func (v *TrieVerifier) verifyTrie(ctx context.Context, trieInfo TrieInfo) error {
	v.logger.Info("Starting trie verification", zap.String("trie", trieInfo.Name))

	expectedRoot := felt.Zero
	err := v.database.View(func(snap db.Snapshot) error {
		reader, err := trieInfo.ReaderFunc(snap, trieInfo.Prefix, trieInfo.Height)
		if err != nil {
			return err
		}
		if reader.RootKey() == nil {
			return nil
		}
		expectedRoot, err = reader.Hash()
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to get stored root hash for %s: %w", trieInfo.Name, err)
	}

	if expectedRoot.IsZero() {
		v.logger.Info("Trie is empty (zero root)", zap.String("trie", trieInfo.Name))
		return nil
	}

	v.logger.Info("Verifying trie",
		zap.String("trie", trieInfo.Name),
		zap.String("expectedRoot", expectedRoot.String()))

	storageReader := trie.NewReadStorage(v.database, trieInfo.Prefix)
	if err := VerifyTrie(ctx, storageReader, trieInfo.Height, trieInfo.HashFn, &expectedRoot); err != nil {
		return err
	}

	v.logger.Info("Trie verification successful",
		zap.String("trie", trieInfo.Name),
		zap.String("root", expectedRoot.String()))
	return nil
}

func (v *TrieVerifier) verifyContractStorageTries(
	ctx context.Context, filterAddress *felt.Felt,
) error {
	var contractAddresses []felt.Felt
	if filterAddress != nil {
		contractAddresses = []felt.Felt{*filterAddress}
	} else {
		var err error
		contractAddresses, err = v.collectContractAddresses()
		if err != nil {
			return err
		}
	}

	v.logger.Info("Starting contract storage tries verification",
		zap.Int("count", len(contractAddresses)))

	for i, contractAddress := range contractAddresses {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		addrBytes := contractAddress.Marshal()
		prefix := db.ContractStorage.Key(addrBytes)
		trieInfo := TrieInfo{
			Name:   fmt.Sprintf("ContractStorage[%s]", contractAddress.String()),
			Prefix: prefix,
			HashFn: crypto.Pedersen,
			ReaderFunc: func(r db.KeyValueReader, _ []byte, height uint8) (trie.TrieReader, error) {
				return trie.NewTrieReaderPedersen(r, prefix, height)
			},
			Height: StarknetTrieHeight,
		}

		v.logger.Info("Verifying contract storage",
			zap.String("contract", contractAddress.String()),
			zap.String("progress", fmt.Sprintf("%d/%d", i+1, len(contractAddresses))))

		if err := v.verifyTrie(ctx, trieInfo); err != nil {
			return err
		}
	}

	v.logger.Info("All contract storage tries verified successfully",
		zap.Int("count", len(contractAddresses)))
	return nil
}
