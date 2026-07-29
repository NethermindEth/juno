package deprecated_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/deprecated"
	"github.com/stretchr/testify/require"
)

func TestBucketMover(t *testing.T) {
	beforeCalled := false
	sourceBucket := db.Bucket(0)
	destBucket := db.Bucket(1)
	mover := deprecated.NewBucketMover(sourceBucket, destBucket).WithBefore(func() {
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
	_, err = mover.Migrate(t.Context(), testDB, &networks.Mainnet, nil)
	require.ErrorIs(t, err, deprecated.ErrCallWithNewTransaction)

	intermediateState, err = mover.Migrate(t.Context(), testDB, &networks.Mainnet, nil)
	require.NoError(t, err)

	snap := testDB.NewSnapshot()
	defer snap.Close()

	require.NoError(t, snap.Get(sourceBucket.Key(), func(data []byte) error {
		require.Equal(t, []byte{44}, data, "shouldnt have changed")
		return nil
	}))

	for i := byte(0); i < 3; i++ {
		require.NoError(t, snap.Get(destBucket.Key([]byte{i}), func(data []byte) error {
			require.Equal(t, []byte{i}, data, "shouldve moved")
			return nil
		}))

		err = snap.Get(sourceBucket.Key([]byte{i}), func([]byte) error { return nil })
		require.ErrorIs(t, db.ErrKeyNotFound, err)
	}
	require.Nil(t, intermediateState)
}
