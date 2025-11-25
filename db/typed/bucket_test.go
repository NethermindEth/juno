package typed_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name   string
	del    int
	update int
	insert int
}

var bucket = typed.NewBucket(
	db.Bucket(0),
	key.Uint64,
	value.Felt,
)

func del(
	t *testing.T,
	txn db.KeyValueWriter,
	del int,
	expected map[uint64]felt.Felt,
) (deleted []uint64) {
	t.Helper()
	t.Run(fmt.Sprintf("Delete %d entries", del), func(t *testing.T) {
		deleted = make([]uint64, 0, del)
		for key := range expected {
			delete(expected, key)
			require.NoError(t, bucket.Delete(txn, key))

			deleted = append(deleted, key)
			if len(deleted) >= del {
				break
			}
		}
	})
	return deleted
}

func update(
	t *testing.T,
	txn db.KeyValueWriter,
	update int,
	expected map[uint64]felt.Felt,
) {
	t.Helper()
	t.Run(fmt.Sprintf("Update %d entries", update), func(t *testing.T) {
		updated := 0
		for key := range expected {
			value := felt.Random[felt.Felt]()
			expected[key] = value
			require.NoError(t, bucket.Put(txn, key, &value))

			updated++
			if updated >= update {
				break
			}
		}
	})
}

func insert(
	t *testing.T,
	txn db.KeyValueWriter,
	insert int,
	expected map[uint64]felt.Felt,
) {
	t.Helper()
	t.Run(fmt.Sprintf("Insert %d entries", insert), func(t *testing.T) {
		for range insert {
			key := rand.Uint64()
			for {
				if _, exists := expected[key]; !exists {
					break
				}
				key = rand.Uint64()
			}
			value := felt.Random[felt.Felt]()
			expected[key] = value
			require.NoError(t, bucket.Put(txn, key, &value))
		}
	})
}

func assertExists(t *testing.T, txn db.KeyValueReader, expected map[uint64]felt.Felt) {
	for key, value := range expected {
		has, err := bucket.Has(txn, key)
		require.NoError(t, err)
		require.True(t, has)

		got, err := bucket.Get(txn, key)
		require.NoError(t, err)
		require.Equal(t, value, got)
	}
}

func assertNotExists(t *testing.T, txn db.KeyValueReader, keys []uint64) {
	for _, key := range keys {
		has, err := bucket.Has(txn, key)
		require.NoError(t, err)
		require.False(t, has)

		_, err = bucket.Get(txn, key)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	}
}

func TestBucket(t *testing.T) {
	cases := []testCase{
		{
			name:   "Insert first entries",
			insert: 10000,
		},
		{
			name:   "Update-only",
			update: 1000,
		},
		{
			name: "Delete-only",
			del:  1000,
		},
		{
			name:   "Insert and update entries",
			insert: 1000,
			update: 1000,
		},
		{
			name:   "Mix writes",
			insert: 1000,
			update: 1000,
			del:    1000,
		},
	}

	database := memory.New()
	expected := make(map[uint64]felt.Felt)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var deleted []uint64
			err := database.Update(func(txn db.IndexedBatch) error {
				if tc.del > 0 {
					deleted = del(t, txn, tc.del, expected)
				}
				if tc.update > 0 {
					update(t, txn, tc.update, expected)
				}
				if tc.insert > 0 {
					insert(t, txn, tc.insert, expected)
				}
				return nil
			})
			require.NoError(t, err)
			assertNotExists(t, database, deleted)
			assertExists(t, database, expected)
		})
	}

	t.Run("RawKey and RawValue", func(t *testing.T) {
		rawBucket := bucket.RawKey().RawValue()

		for k, v := range expected {
			keyBytes := key.Uint64.Marshal(k)
			has, err := rawBucket.Has(database, keyBytes)
			require.NoError(t, err)
			require.True(t, has)

			expectedBytes, err := value.Felt.Marshal(&v)
			require.NoError(t, err)

			actualBytes, err := rawBucket.Get(database, keyBytes)
			require.NoError(t, err)
			require.Equal(t, expectedBytes, actualBytes)
		}
	})
}
