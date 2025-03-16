package migration_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/utils"
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

	testDB := memory.New()
	require.NoError(t, testDB.Update(func(txn db.IndexedBatch) error {
		for i := byte(0); i < 3; i++ {
			if err := txn.Put(sourceBucket.Key([]byte{i}), []byte{i}); err != nil {
				return err
			}
		}
		return txn.Put(sourceBucket.Key(), []byte{44})
	}))

	require.NoError(t, mover.Before(nil))
	require.True(t, beforeCalled)
	var (
		intermediateState []byte
		err               error
	)
	_, err = mover.Migrate(context.Background(), testDB, &utils.Mainnet, nil)
	require.ErrorIs(t, err, migration.ErrCallWithNewTransaction)

	intermediateState, err = mover.Migrate(context.Background(), testDB, &utils.Mainnet, nil)
	require.NoError(t, err)

	err = testDB.View(func(txn db.Snapshot) error {
		var val []byte
		err := txn.Get(sourceBucket.Key(), func(data []byte) error {
			val = data
			return nil
		})
		if err != nil {
			return err
		}
		if !bytes.Equal(val, []byte{44}) {
			return errors.New("shouldnt have changed")
		}

		for i := byte(0); i < 3; i++ {
			var val []byte
			err := txn.Get(destBucket.Key([]byte{i}), func(data []byte) error {
				val = data
				return nil
			})
			if err != nil {
				return err
			}
			if !bytes.Equal(val, []byte{i}) {
				return errors.New("shouldve moved")
			}

			err = txn.Get(sourceBucket.Key([]byte{i}), func([]byte) error { return nil })
			require.ErrorIs(t, db.ErrKeyNotFound, err)
		}
		return nil
	})
	require.NoError(t, err)
	require.Nil(t, intermediateState)
}
