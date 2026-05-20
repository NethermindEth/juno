package trie

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
)

const (
	batchByteSize         = 128 * db.Megabyte
	targetBatchByteSize   = 96 * db.Megabyte
	timeLogRate           = 5 * time.Second
	SmallTrieThreshold    = 100_000
	parallelHashBatchSize = 16384
	IngestorCount         = 4
)

var (
	shouldRerun    = []byte{}
	shouldNotRerun []byte
)

var deprecatedTrieBuckets = []db.Bucket{db.ClassesTrie, db.StateTrie, db.ContractStorage}

type Migrator struct{}

var _ migration.Migration = (*Migrator)(nil)

func (*Migrator) Before([]byte) error { return nil }

func (*Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	needed, err := needsMigration(database)
	if err != nil {
		return shouldRerun, err
	}
	if !needed {
		logger.Info("trie migration: no old-format data found, marking applied")
		return shouldNotRerun, nil
	}

	return runMigration(ctx, database, logger)
}

func needsMigration(r db.KeyValueReader) (bool, error) {
	for _, bucket := range deprecatedTrieBuckets {
		prefix := bucket.Key()
		iter, err := r.NewIterator(prefix, true)
		if err != nil {
			return false, err
		}
		hasKeys := iter.First()
		if err := iter.Close(); err != nil {
			return false, err
		}
		if hasKeys {
			return true, nil
		}
	}
	return false, nil
}

func runMigration(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) ([]byte, error) {
	batchSem := semaphore.New(IngestorCount*2, func() db.Batch {
		return database.NewBatchWithSize(batchByteSize)
	})

	pool := newHashWorkerPool()
	defer pool.close()

	ing := newIngestor(ctx, database, batchSem, pool)

	tries, err := enumerateTries(database)
	if err != nil {
		return shouldRerun, err
	}

	var allNodes uint64
	for _, d := range tries {
		allNodes += uint64(d.NodeCount)
	}
	allTries := uint64(len(tries))

	src := pipeline.Source(func(yield func(TrieDesc) bool) {
		for _, d := range tries {
			if !yield(d) {
				return
			}
		}
	})
	ingested := pipeline.New(src, IngestorCount, ing)
	committed := pipeline.New(
		ingested,
		1,
		newCommitter(logger, batchSem, allTries, allNodes),
	)

	_, wait := committed.Run(ctx)
	res := wait()
	if res.Err != nil {
		return shouldRerun, res.Err
	}
	if !res.IsDone {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return shouldRerun, ctxErr
		}
		return shouldRerun, errors.New("trie migration: pipeline did not complete")
	}

	if err := wipeDeprecatedBuckets(database); err != nil {
		return shouldRerun, err
	}
	logger.Info("trie migration: source buckets deleted")

	return shouldNotRerun, nil
}

func wipeDeprecatedBuckets(database db.KeyValueRangeDeleter) error {
	for _, bucket := range deprecatedTrieBuckets {
		start := bucket.Key()
		end := dbutils.UpperBound(start)
		if err := database.DeleteRange(start, end); err != nil {
			return fmt.Errorf("trie migration: cleanup DeleteRange for %v: %w", bucket, err)
		}
	}
	return nil
}

type TrieDesc struct {
	DeprecatedTrieBucket db.Bucket
	TrieBucket           db.Bucket
	Owner                felt.Address
	HashFn               crypto.HashFn
	NodeCount            int
	RootPath             *trie.BitArray
}

func enumerateTries(r db.KeyValueReader) ([]TrieDesc, error) {
	var descs []TrieDesc

	for _, spec := range []struct {
		oldBucket, newBucket db.Bucket
		hashFn               crypto.HashFn
	}{
		{db.ClassesTrie, db.ClassTrie, crypto.Poseidon},
		{db.StateTrie, db.ContractTrieContract, crypto.Pedersen},
	} {
		desc, err := enumerateGlobalTrie(r, spec.oldBucket, spec.newBucket, spec.hashFn)
		if err != nil {
			return nil, err
		}
		descs = append(descs, desc)
	}

	storageDescs, err := enumerateStorageTries(r)
	if err != nil {
		return nil, err
	}
	descs = append(descs, storageDescs...)

	return descs, nil
}

func enumerateGlobalTrie(
	r db.KeyValueReader,
	oldBucket, newBucket db.Bucket,
	hashFn crypto.HashFn,
) (TrieDesc, error) {
	prefix := oldBucket.Key()
	it, err := r.NewIterator(prefix, true)
	if err != nil {
		return TrieDesc{}, fmt.Errorf("opening iterator for bucket %v: %w", oldBucket, err)
	}
	defer it.Close()
	it.First()

	rootPath, count, err := scanTrie(it, prefix)
	if err != nil {
		return TrieDesc{}, fmt.Errorf("enumerating bucket %v: %w", oldBucket, err)
	}
	return TrieDesc{
		DeprecatedTrieBucket: oldBucket,
		TrieBucket:           newBucket,
		HashFn:               hashFn,
		NodeCount:            count,
		RootPath:             &rootPath,
	}, nil
}

func enumerateStorageTries(r db.KeyValueReader) ([]TrieDesc, error) {
	it, err := r.NewIterator(db.ContractStorage.Key(), true)
	if err != nil {
		return nil, fmt.Errorf("opening storage iterator: %w", err)
	}
	defer it.Close()
	it.First()

	var descs []TrieDesc
	for it.Valid() {
		key := it.Key()
		if len(key) < 1+felt.Bytes {
			it.Next()
			continue
		}
		owner := felt.FromBytes[felt.Address](key[1 : 1+felt.Bytes])
		ownerBytes := owner.Bytes()
		ownerPrefix := db.ContractStorage.Key(ownerBytes[:])

		rootPath, count, err := scanTrie(it, ownerPrefix)
		if err != nil {
			return nil, fmt.Errorf("enumerating storage owner %s: %w", &owner, err)
		}
		descs = append(descs, TrieDesc{
			DeprecatedTrieBucket: db.ContractStorage,
			TrieBucket:           db.ContractTrieStorage,
			Owner:                owner,
			HashFn:               crypto.Pedersen,
			NodeCount:            count,
			RootPath:             &rootPath,
		})
		// scanTrie leaves the iterator positioned past this owner's range.
	}
	return descs, nil
}

func scanTrie(it db.Iterator, prefix []byte) (trie.BitArray, int, error) {
	var rootPath trie.BitArray
	count := 0
	for it.Valid() {
		key := it.Key()
		if !bytes.HasPrefix(key, prefix) {
			return rootPath, count, nil
		}
		if len(key) == len(prefix) {
			val, err := it.Value()
			if err != nil {
				return trie.BitArray{}, 0, err
			}
			parsedRootPath, err := parseRootPath(val)
			if err != nil {
				return trie.BitArray{}, 0, err
			}
			rootPath = parsedRootPath
		} else {
			count++
		}
		it.Next()
	}
	return rootPath, count, nil
}

func parseRootPath(val []byte) (trie.BitArray, error) {
	if len(val) == 0 {
		return trie.BitArray{}, nil
	}
	var ba trie.BitArray
	if err := ba.UnmarshalBinary(val); err != nil {
		return trie.BitArray{}, err
	}
	return ba, nil
}
