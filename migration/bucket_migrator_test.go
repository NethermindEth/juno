package migration_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/migration"
	"github.com/stretchr/testify/require"
)

func TestBucketMover(t *testing.T) {
	beforeCalled := false
	sourceBucket := db.Bucket(0)
	destBucket := db.Bucket(1)
	mover := migration.NewBucketMover(sourceBucket, destBucket).WithBefore(func() {
		beforeCalled = true
	}).WithBatchSize(2).WithKeyFilter(func(b []byte) (bool, error) {
		return len(b) > 1, nil
	})

	testDB := pebble.NewMemTest()
	require.NoError(t, testDB.Update(func(txn db.Transaction) error {
		for i := byte(0); i < 3; i++ {
			if err := txn.Set(sourceBucket.Key([]byte{i}), []byte{i}); err != nil {
				return err
			}
		}
		return txn.Set(sourceBucket.Key(), []byte{44})
	}))

	mover.Before()
	require.True(t, beforeCalled)

	err := testDB.Update(func(txn db.Transaction) error {
		err := mover.Migrate(txn)
		require.ErrorIs(t, err, migration.ErrCallWithNewTransaction)
		return nil
	})
	require.NoError(t, err)
	err = testDB.Update(func(txn db.Transaction) error {
		err = mover.Migrate(txn)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	err = testDB.View(func(txn db.Transaction) error {
		err = txn.Get(sourceBucket.Key(), func(b []byte) error {
			if !bytes.Equal(b, []byte{44}) {
				return errors.New("shouldnt have changed")
			}
			return nil
		})
		if err != nil {
			return err
		}

		for i := byte(0); i < 3; i++ {
			err = txn.Get(destBucket.Key([]byte{i}), func(b []byte) error {
				if !bytes.Equal(b, []byte{i}) {
					return errors.New("shouldve moved")
				}
				return nil
			})
			if err != nil {
				return err
			}

			err = txn.Get(sourceBucket.Key([]byte{i}), func(b []byte) error { return nil })
			require.ErrorIs(t, db.ErrKeyNotFound, err)
		}
		return nil
	})
	require.NoError(t, err)
}
