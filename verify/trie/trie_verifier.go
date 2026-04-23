package trie

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

var (
	stateTrieInfo = TrieInfo{
		Name:   "ContractsTrie",
		Prefix: db.StateTrie.Key(),
		HashFn: crypto.Pedersen,
		ReaderFunc: func(r db.KeyValueReader, height uint8) (trie.TrieReader, error) {
			return trie.NewTrieReaderPedersen(r, db.StateTrie.Key(), height)
		},
		Height: StarknetTrieHeight,
	}

	classTrieInfo = TrieInfo{
		Name:   "ClassesTrie",
		Prefix: db.ClassesTrie.Key(),
		HashFn: crypto.Poseidon,
		ReaderFunc: func(r db.KeyValueReader, height uint8) (trie.TrieReader, error) {
			return trie.NewTrieReaderPoseidon(r, db.ClassesTrie.Key(), height)
		},
		Height: StarknetTrieHeight,
	}
)

type TrieVerifier struct {
	database        db.KeyValueStore
	logger          log.StructuredLogger
	trieTypes       []TrieType
	contractAddress *felt.Felt
}

func NewTrieVerifier(
	database db.KeyValueStore,
	logger log.StructuredLogger,
	trieTypes []TrieType,
	contractAddress *felt.Felt,
) *TrieVerifier {
	if len(trieTypes) == 0 {
		trieTypes = allTrieTypes
	}
	return &TrieVerifier{
		database:        database,
		logger:          logger,
		trieTypes:       trieTypes,
		contractAddress: contractAddress,
	}
}

func (v *TrieVerifier) Name() string {
	return "trie"
}

func (v *TrieVerifier) Run(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		v.logger.Info("Trie verification finished",
			zap.Duration("total_elapsed", time.Since(startTime).Round(time.Second)))
	}()

	err := v.database.View(func(snap db.Snapshot) error {
		return v.verifyAll(ctx, snap)
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if err != nil {
		return err
	}

	v.logger.Info("Trie verification completed successfully")
	return nil
}

func (v *TrieVerifier) verifyAll(ctx context.Context, snap db.Snapshot) error {
	for _, t := range v.trieTypes {
		var (
			name string
			err  error
		)
		switch t {
		case ContractTrie:
			name, err = stateTrieInfo.Name, v.verifyTrie(ctx, snap, stateTrieInfo)
		case ClassTrie:
			name, err = classTrieInfo.Name, v.verifyTrie(ctx, snap, classTrieInfo)
		case ContractStorageTrie:
			name, err = "ContractStorageTries", v.verifyContractStorageTries(ctx, snap, v.contractAddress)
		}
		if err != nil {
			v.logResult(err, name)
			return err
		}
	}
	return nil
}

func (v *TrieVerifier) logResult(err error, trieName string) {
	switch {
	case errors.Is(err, context.Canceled):
		v.logger.Info("Verification stopped", zap.String("trie", trieName))
	case errors.Is(err, ErrCorruptionDetected):
		v.logger.Error("Corruption detected",
			zap.String("trie", trieName),
			zap.String("details", err.Error()))
	default:
		v.logger.Error("Verification error",
			zap.String("trie", trieName),
			zap.Error(err))
	}
}

func contractStorageTrieInfo(addr *felt.Felt) TrieInfo {
	prefix := db.ContractStorage.Key(addr.Marshal())
	return TrieInfo{
		Name:   fmt.Sprintf("ContractStorage[%s]", addr.String()),
		Prefix: prefix,
		HashFn: crypto.Pedersen,
		ReaderFunc: func(r db.KeyValueReader, height uint8) (trie.TrieReader, error) {
			return trie.NewTrieReaderPedersen(r, prefix, height)
		},
		Height: StarknetTrieHeight,
	}
}

func (v *TrieVerifier) verifyTrie(ctx context.Context, snap db.Snapshot, trieInfo TrieInfo) error {
	v.logger.Info("Starting trie verification", zap.String("trie", trieInfo.Name))

	reader, err := trieInfo.ReaderFunc(snap, trieInfo.Height)
	if err != nil {
		return fmt.Errorf("failed to open reader for %s: %w", trieInfo.Name, err)
	}
	if reader.RootKey() == nil {
		v.logger.Info("Trie is empty", zap.String("trie", trieInfo.Name))
		return nil
	}

	expectedRoot, err := reader.Hash()
	if err != nil {
		return fmt.Errorf("failed to get stored root hash for %s: %w", trieInfo.Name, err)
	}

	v.logger.Info("Verifying trie",
		zap.String("trie", trieInfo.Name),
		zap.String("expectedRoot", expectedRoot.String()))

	storageReader := trie.NewReadStorage(snap, trieInfo.Prefix)
	if err := VerifyTrie(
		ctx,
		storageReader,
		trieInfo.Height,
		trieInfo.HashFn,
		&expectedRoot,
	); err != nil {
		return err
	}

	v.logger.Info("Trie verification successful",
		zap.String("trie", trieInfo.Name),
		zap.String("root", expectedRoot.String()))
	return nil
}

func (v *TrieVerifier) verifyContractStorageTries(
	ctx context.Context, snap db.Snapshot, filterAddress *felt.Felt,
) error {
	if filterAddress != nil {
		return v.verifyTrie(ctx, snap, contractStorageTrieInfo(filterAddress))
	}

	bucketPrefix := db.ContractStorage.Key()
	it, err := snap.NewIterator(bucketPrefix, true)
	if err != nil {
		return fmt.Errorf("failed to open contract storage iterator: %w", err)
	}
	defer it.Close()

	v.logger.Info("Starting contract storage tries verification")

	count := 0
	addrStart := len(bucketPrefix)
	addrEnd := addrStart + felt.Bytes

	for ok := it.First(); ok; {
		if err := ctx.Err(); err != nil {
			return err
		}

		key := it.Key()
		if len(key) < addrEnd {
			// Unexpected short key in bucket — skip it.
			ok = it.Next()
			continue
		}
		addrBytes := key[addrStart:addrEnd]

		var addr felt.Felt
		addr.SetBytes(addrBytes)

		count++
		v.logger.Info("Verifying contract storage",
			zap.String("contract", addr.String()),
			zap.Int("index", count))

		if err := v.verifyTrie(ctx, snap, contractStorageTrieInfo(&addr)); err != nil {
			return err
		}

		nextAddr := nextLexAddr(addrBytes)
		if nextAddr == nil {
			break
		}
		ok = it.Seek(db.ContractStorage.Key(nextAddr))
	}

	v.logger.Info("All contract storage tries verified successfully",
		zap.Int("count", count))
	return nil
}

// nextLexAddr returns the lexicographically next 32-byte address, or nil if
// addr is the maximum value (all 0xff).
func nextLexAddr(addr []byte) []byte {
	out := make([]byte, len(addr))
	copy(out, addr)
	for i := len(out) - 1; i >= 0; i-- {
		out[i]++
		if out[i] != 0 {
			return out
		}
	}
	return nil
}
