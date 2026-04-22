package core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/db"
)

// Trie buckets unique to each state implementation. A non-empty bucket from
// either set is a positive signal that the DB was built with that scheme.
var (
	oldStateTrieBuckets = []db.Bucket{db.StateTrie, db.ClassesTrie, db.ContractStorage}
	newStateTrieBuckets = []db.Bucket{db.ContractTrieContract, db.ContractTrieStorage, db.ClassTrie}
)

// ValidateStateVersion checks that the state implementation selected by useNewState
// is consistent with the data already stored in disk. It must be called before any
// state reads or writes so that incompatible databases are rejected early.
//
// It returns an error if:
//   - the database was built with the deprecated state and --new-state is set, or
//   - the database was built with the new state and --new-state is not set.
//
// An empty database is always accepted.
func ValidateStateVersion(disk db.KeyValueStore, useNewState bool) error {
	// A fresh database has no conflict.
	if _, err := GetChainHeight(disk); errors.Is(err, db.ErrKeyNotFound) {
		return nil
	}

	hasNewState, err := anyBucketHasData(disk, newStateTrieBuckets)
	if err != nil {
		return fmt.Errorf("probe new-state buckets: %w", err)
	}
	if !useNewState && hasNewState {
		return errors.New(
			"database was built with the new state implementation, " +
				"but --new-state is not set; add --new-state to use this database",
		)
	}

	hasOldState, err := anyBucketHasData(disk, oldStateTrieBuckets)
	if err != nil {
		return fmt.Errorf("probe state buckets: %w", err)
	}
	if useNewState && hasOldState {
		return errors.New(
			"database was built with the existing state implementation, " +
				"but --new-state is set; remove --new-state to use this database",
		)
	}
	return nil
}

func anyBucketHasData(disk db.KeyValueStore, buckets []db.Bucket) (bool, error) {
	for _, b := range buckets {
		ok, err := bucketHasData(disk, b)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func bucketHasData(disk db.KeyValueStore, bucket db.Bucket) (bool, error) {
	it, err := disk.NewIterator(bucket.Key(), true)
	if err != nil {
		return false, err
	}
	defer it.Close()
	return it.Next(), nil
}
