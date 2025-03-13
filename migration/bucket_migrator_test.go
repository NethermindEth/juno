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
	mover := migration.NewBucketMover2(sourceBucket, destBucket).WithBefore(func() {
		beforeCalled = true
	}).WithBatchSize(2).WithKeyFilter(func(b []byte) (bool, error) {
		return len(b) > 1, nil
	})

	testDB := memory.New()
	require.NoError(t, testDB.Update2(func(txn db.IndexedBatch) error {
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
	err = testDB.Update2(func(txn db.IndexedBatch) error {
		intermediateState, err = mover.Migrate(context.Background(), txn, &utils.Mainnet, nil)
		require.ErrorIs(t, err, migration.ErrCallWithNewTransaction)
		return nil
	})
	require.NoError(t, err)
	err = testDB.Update2(func(txn db.IndexedBatch) error {
		intermediateState, err = mover.Migrate(context.Background(), txn, &utils.Mainnet, nil)
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	err = testDB.View2(func(txn db.Snapshot) error {
		val, err := txn.Get2(sourceBucket.Key())
		if err != nil {
			return err
		}
		if !bytes.Equal(val, []byte{44}) {
			return errors.New("shouldnt have changed")
		}

		if err != nil {
			return err
		}

		for i := byte(0); i < 3; i++ {
			val, err := txn.Get2(destBucket.Key([]byte{i}))
			if err != nil {
				return err
			}
			if !bytes.Equal(val, []byte{i}) {
				return errors.New("shouldve moved")
			}

			_, err = txn.Get2(sourceBucket.Key([]byte{i}))
			require.ErrorIs(t, db.ErrKeyNotFound, err)
		}
		return nil
	})
	require.NoError(t, err)
	require.Nil(t, intermediateState)
}
