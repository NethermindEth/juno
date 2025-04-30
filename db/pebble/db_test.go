package pebble

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPebbleDB(t *testing.T) {
	t.Run("test suite", func(t *testing.T) {
		db.TestKeyValueStoreSuite(t, func() db.KeyValueStore {
			db, err := pebble.Open("", &pebble.Options{
				FS: vfs.NewMem(),
			})
			require.NoError(t, err)
			return &DB{
				db:        db,
				closeLock: new(sync.RWMutex),
				listener:  &eventListener{},
			}
		})
	})
}

type eventListener struct {
	WriteCount int
	ReadCount  int
}

func (l *eventListener) OnIO(write bool, _ time.Duration) {
	if write {
		l.WriteCount++
	} else {
		l.ReadCount++
	}
}

func (l *eventListener) OnCommit(_ time.Duration) {}

func newPebbleMem() (*DB, error) {
	db, err := pebble.Open("", &pebble.Options{
		FS: vfs.NewMem(),
	})
	if err != nil {
		return nil, err
	}

	return &DB{
		db:        db,
		closeLock: new(sync.RWMutex),
	}, nil
}

func TestCalculatePrefixSize(t *testing.T) {
	t.Run("empty db", func(t *testing.T) {
		testDB, err := newPebbleMem()
		require.NoError(t, err)

		s, err := CalculatePrefixSize(t.Context(), testDB, []byte("0"), true)
		require.NoError(t, err)
		assert.Zero(t, s.Count)
		assert.Zero(t, s.Size)
	})

	t.Run("non empty db but empty prefix", func(t *testing.T) {
		testDB, err := newPebbleMem()
		require.NoError(t, err)
		require.NoError(t, testDB.Put(append([]byte("0"), []byte("randomKey")...), []byte("someValue")))
		s, err := CalculatePrefixSize(t.Context(), testDB, []byte("1"), true)
		require.NoError(t, err)
		assert.Zero(t, s.Count)
		assert.Zero(t, s.Size)
	})

	t.Run("size of all key value pair with the same prefix", func(t *testing.T) {
		p := []byte("0")
		k1, v1 := append(p, []byte("key1")...), []byte("value1") //nolint: gocritic
		k2, v2 := append(p, []byte("key2")...), []byte("value2") //nolint: gocritic
		k3, v3 := append(p, []byte("key3")...), []byte("value3") //nolint: gocritic
		expectedSize := uint(len(k1) + len(v1) + len(k2) + len(v2) + len(k3) + len(v3))

		testDB, err := newPebbleMem()
		require.NoError(t, err)
		require.NoError(t, testDB.Put(k1, v1))
		require.NoError(t, testDB.Put(k2, v2))
		require.NoError(t, testDB.Put(k3, v3))

		s, err := CalculatePrefixSize(t.Context(), testDB, p, true)
		require.NoError(t, err)
		assert.Equal(t, uint(3), s.Count)
		assert.Equal(t, utils.DataSize(expectedSize), s.Size)

		t.Run("exit when context is cancelled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			s, err := CalculatePrefixSize(ctx, testDB, p, true)
			assert.EqualError(t, err, context.Canceled.Error())
			assert.Zero(t, s.Count)
			assert.Zero(t, s.Size)
		})
	})
}
