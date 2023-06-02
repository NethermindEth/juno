package migration

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/db"
)

type revision func(transaction db.Transaction) error

// Revisions list contains a set of revisions that can be applied to a database
// After making breaking changes to the DB layout, add new revisions to this list.
var revisions = []revision{
	revision0000,
	relocateContractStorageRootKeys,
}

func MigrateIfNeeded(targetDB db.DB) error {
	/*
		Schema version of the targetDB determines which set of revisions need to be applied to the database.
		After revision is successfully executed, which may update the database, schema version is incremented
		by 1 by this loop.

		For example;

		Assume an empty DB, which has a schema version of 0. So we start applying the revisions from the
		very first one in the list and increment the schema version as we go. After the loop exists we
		end up with a database which's schema version is len(revisions).

		After running that Juno version for a while, if the user updates its Juno version which adds new
		revisions to the list, MigrateIfNeeded will skip the already applied revisions and only apply the
		new ones. It will be able to do this since the schema version it reads from the database will be
		non-zero and that is what we use to initialise the i loop variable.
	*/
	version, err := SchemaVersion(targetDB)
	if err != nil {
		return err
	}

	for i := version; i < uint64(len(revisions)); i++ {
		if err = targetDB.Update(func(txn db.Transaction) error {
			if err = revisions[i](txn); err != nil {
				return err
			}

			// revision returned with no errors, bump the version
			var versionBytes [8]byte
			binary.BigEndian.PutUint64(versionBytes[:], i+1)
			return txn.Set(db.SchemaVersion.Key(), versionBytes[:])
		}); err != nil {
			return err
		}
	}

	return nil
}

func SchemaVersion(targetDB db.DB) (uint64, error) {
	version := uint64(0)
	txn := targetDB.NewTransaction(false)
	err := txn.Get(db.SchemaVersion.Key(), func(bytes []byte) error {
		version = binary.BigEndian.Uint64(bytes)
		return nil
	})
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return 0, db.CloseAndWrapOnError(txn.Discard, err)
	}

	return version, db.CloseAndWrapOnError(txn.Discard, nil)
}

// revision0000 makes sure the targetDB is empty
func revision0000(txn db.Transaction) error {
	it, err := txn.NewIterator()
	if err != nil {
		return err
	}

	// not empty if valid
	if it.Next() {
		return db.CloseAndWrapOnError(it.Close, errors.New("initial DB should be empty"))
	}
	return it.Close()
}

// relocateContractStorageRootKeys moves contract root keys to new locations and deletes the previous locations.
//
// Before: the key to the root in a contract storage trie was stored at 1+<contractAddress>.
// After: the key to the root of the contract storage trie is stored at 3+<contractAddress>.
//
// This enables us to remove the db.ContractRootKey prefix.
func relocateContractStorageRootKeys(txn db.Transaction) error {
	it, err := txn.NewIterator()
	if err != nil {
		return err
	}

	// Modifying the db with txn.Set/Delete while iterating can cause consistency issues,
	// so we do them separately.

	// Iterate over all entries in the old bucket, copying each into memory.
	// Even with millions of contracts, this shouldn't be too expensive.
	oldEntries := make(map[string][]byte)
	oldPrefix := db.Unused.Key()
	var value []byte
	for it.Seek(oldPrefix); it.Valid(); it.Next() {
		// Stop iterating once we're out of the old bucket.
		if !bytes.Equal(it.Key()[:len(oldPrefix)], oldPrefix) {
			break
		}

		oldKey := it.Key()
		value, err = it.Value()
		if err != nil {
			return db.CloseAndWrapOnError(it.Close, err)
		}
		oldEntries[string(oldKey)] = value
	}

	if err = it.Close(); err != nil {
		return err
	}

	// Move the entries to their new locations.
	for oldKey, value := range oldEntries {
		oldKeyBytes := []byte(oldKey)
		contractAddress, found := bytes.CutPrefix(oldKeyBytes, oldPrefix)
		if !found {
			// Should not happen.
			return db.CloseAndWrapOnError(it.Close, errors.New("prefix not found"))
		}

		if err := txn.Set(db.ContractStorage.Key(contractAddress), value); err != nil {
			return db.CloseAndWrapOnError(it.Close, err)
		}
		if err := txn.Delete(oldKeyBytes); err != nil {
			return db.CloseAndWrapOnError(it.Close, err)
		}
	}

	return nil
}
