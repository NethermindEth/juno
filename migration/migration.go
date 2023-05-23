package migration

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/db"
)

type revision func(transaction db.Transaction) error

// Revisions list contains a set of revisions that can be applied to a database
// After making breaking changes to the DB layout, add new revisions to this list.
var revisions = []revision{
	revision0000,
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
