package blockchain

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
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
	if _, err := core.GetChainHeight(disk); errors.Is(err, db.ErrKeyNotFound) {
		return nil
	}

	hasNewState, err := bucketHasData(disk, db.ClassTrie)
	if err != nil {
		return fmt.Errorf("probe new-state bucket: %w", err)
	}
	hasOldState, err := bucketHasData(disk, db.ClassesTrie)
	if err != nil {
		return fmt.Errorf("probe state bucket: %w", err)
	}

	switch {
	case useNewState && hasOldState:
		return errors.New(
			"database was built with the existing state implementation, " +
				"but --new-state is set; remove --new-state to use this database",
		)
	case !useNewState && hasNewState:
		return errors.New(
			"database was built with the new state implementation, " +
				"but --new-state is not set; add --new-state to use this database",
		)
	}
	return nil
}

func bucketHasData(disk db.KeyValueStore, bucket db.Bucket) (bool, error) {
	it, err := disk.NewIterator(bucket.Key(), false)
	if err != nil {
		return false, err
	}
	defer it.Close()
	return it.Next(), nil
}
