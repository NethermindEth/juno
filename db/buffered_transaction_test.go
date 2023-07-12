package db_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/require"
)

func TestBufferedTransaction(t *testing.T) {
	txn := db.NewMemTransaction()

	existingKey := []byte("existingKey")
	require.NoError(t, txn.Set(existingKey, []byte("existingValue")))

	bufferedTxn := db.NewBufferedTransaction(txn)
	require.NoError(t, bufferedTxn.Delete(existingKey))
	newKey := []byte("newKey")
	require.NoError(t, bufferedTxn.Set(newKey, []byte("newValue")))

	t.Run("buffered changes should be visible on BufferedTransaction", func(t *testing.T) {
		require.ErrorIs(t, bufferedTxn.Get(existingKey, nil), db.ErrKeyNotFound)
		require.NoError(t, bufferedTxn.Get(newKey, func(b []byte) error {
			if string(b) != "newValue" {
				return errors.New("not expected value")
			}
			return nil
		}))
	})

	t.Run("buffered changes should not be visible on the original transaction", func(t *testing.T) {
		require.ErrorIs(t, txn.Get(newKey, nil), db.ErrKeyNotFound)
		require.NoError(t, txn.Get(existingKey, func(b []byte) error {
			if string(b) != "existingValue" {
				return errors.New("not expected value")
			}
			return nil
		}))
	})

	require.NoError(t, bufferedTxn.Commit())

	t.Run("flushed changes should be visible on the original transaction", func(t *testing.T) {
		require.ErrorIs(t, txn.Get(existingKey, nil), db.ErrKeyNotFound)
		require.NoError(t, txn.Get(newKey, func(b []byte) error {
			if string(b) != "newValue" {
				return errors.New("not expected value")
			}
			return nil
		}))
	})
}
