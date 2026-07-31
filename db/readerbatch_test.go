package db_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestReaderBatchReads(t *testing.T) {
	stored := memory.New()
	require.NoError(t, stored.Put([]byte("key"), []byte("value")))

	batch := db.NewReaderBatch(stored)

	t.Run("Get delegates to the wrapped reader", func(t *testing.T) {
		require.Equal(t, []byte("value"), get(t, batch, []byte("key")))
	})

	t.Run("Get of a missing key", func(t *testing.T) {
		err := batch.Get([]byte("missing"), func([]byte) error { return nil })
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("Has delegates to the wrapped reader", func(t *testing.T) {
		has, err := batch.Has([]byte("key"))
		require.NoError(t, err)
		require.True(t, has)

		has, err = batch.Has([]byte("missing"))
		require.NoError(t, err)
		require.False(t, has)
	})

	t.Run("NewIterator delegates to the wrapped reader", func(t *testing.T) {
		it, err := batch.NewIterator([]byte("key"), true)
		require.NoError(t, err)
		defer func() { require.NoError(t, it.Close()) }()

		require.True(t, it.Next())
		value, err := it.Value()
		require.NoError(t, err)
		require.Equal(t, []byte("value"), value)
		require.False(t, it.Next())
	})
}

func TestReaderBatchOverlay(t *testing.T) {
	t.Run("Put is readable back", func(t *testing.T) {
		batch := db.NewReaderBatch(memory.New())
		require.NoError(t, batch.Put([]byte("key"), []byte("buffered")))
		require.Equal(t, []byte("buffered"), get(t, batch, []byte("key")))

		has, err := batch.Has([]byte("key"))
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("Put shadows the wrapped reader", func(t *testing.T) {
		stored := memory.New()
		require.NoError(t, stored.Put([]byte("key"), []byte("value")))

		batch := db.NewReaderBatch(stored)
		require.NoError(t, batch.Put([]byte("key"), []byte("buffered")))
		require.Equal(t, []byte("buffered"), get(t, batch, []byte("key")))
	})

	t.Run("Put does not reach the wrapped store", func(t *testing.T) {
		stored := memory.New()
		batch := db.NewReaderBatch(stored)
		require.NoError(t, batch.Put([]byte("key"), []byte("buffered")))

		err := stored.Get([]byte("key"), func([]byte) error { return nil })
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("Delete masks a stored key", func(t *testing.T) {
		stored := memory.New()
		require.NoError(t, stored.Put([]byte("key"), []byte("value")))

		batch := db.NewReaderBatch(stored)
		require.NoError(t, batch.Delete([]byte("key")))

		err := batch.Get([]byte("key"), func([]byte) error { return nil })
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		has, err := batch.Has([]byte("key"))
		require.NoError(t, err)
		require.False(t, has)
	})
}

func TestReaderBatchClose(t *testing.T) {
	stored := memory.New()
	require.NoError(t, stored.Put([]byte("key"), []byte("value")))

	batch := db.NewReaderBatch(stored)
	require.NoError(t, batch.Close())

	// The wrapped reader's lifetime is owned by the caller, so it stays usable.
	require.Equal(t, []byte("value"), get(t, stored, []byte("key")))
}

func TestReaderBatchUnsupported(t *testing.T) {
	batch := db.NewReaderBatch(memory.New())

	require.Panics(t, func() { _ = batch.Write() })
	require.Panics(t, func() { _ = batch.DeleteRange(nil, nil) })
	require.Panics(t, func() { _ = batch.Size() })
}

func get(t *testing.T, r db.KeyValueReader, key []byte) []byte {
	t.Helper()
	var value []byte
	require.NoError(t, r.Get(key, func(v []byte) error {
		value = append([]byte{}, v...)
		return nil
	}))
	return value
}
