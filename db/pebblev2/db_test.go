package pebblev2

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPebbleDB(t *testing.T) {
	t.Run("test suite", func(t *testing.T) {
		db.TestKeyValueStoreSuite(t, func() db.KeyValueStore {
			return newPebbleMem(t)
		})
	})
}

func newPebbleMem(t *testing.T) *DB {
	db, err := New(t.TempDir(), func(opts *pebble.Options) error {
		opts.FS = vfs.NewMem()
		return nil
	})
	require.NoError(t, err)
	return db.(*DB)
}

func TestCompactAll(t *testing.T) {
	testDB := newPebbleMem(t)

	keys := [][]byte{{0}, {1, 2, 3}, {0xfe}, {0xff, 0xff}}
	for _, key := range keys {
		require.NoError(t, testDB.Put(key, key))
	}

	require.NoError(t, testDB.CompactAll(t.Context(), false))

	for _, key := range keys {
		require.NoError(t, testDB.Get(key, func(value []byte) error {
			assert.Equal(t, key, value)
			return nil
		}))
	}
}

func TestCompactAllForce(t *testing.T) {
	dir := t.TempDir()
	keys := [][]byte{{0}, {1, 2, 3}, {0xfe}, {0xff, 0xff}}

	// Fully compact without a filter policy: the flat bottom level has no
	// filter blocks.
	testDB, err := New(dir)
	require.NoError(t, err)
	for _, key := range keys {
		require.NoError(t, testDB.Put(key, key))
	}
	require.NoError(t, testDB.(*DB).CompactAll(t.Context(), false))
	require.Empty(t, bottomFilterPolicies(t, testDB.(*DB)))
	require.NoError(t, testDB.Close())

	// A plain compaction of the already-flat database is a no-op, force
	// rewrites it with the now-configured filter policy.
	testDB, err = New(dir, WithBloomFilter())
	require.NoError(t, err)
	pDB := testDB.(*DB)

	require.NoError(t, pDB.CompactAll(t.Context(), false))
	require.Empty(t, bottomFilterPolicies(t, pDB))

	require.NoError(t, pDB.CompactAll(t.Context(), true))
	policies := bottomFilterPolicies(t, pDB)
	require.NotEmpty(t, policies)
	for _, policy := range policies {
		assert.Equal(t, "rocksdb.BuiltinBloomFilter", policy)
	}

	for _, key := range keys {
		require.NoError(t, pDB.Get(key, func(value []byte) error {
			assert.Equal(t, key, value)
			return nil
		}))
	}
	require.NoError(t, pDB.Close())
}

func TestCompactAllForceRewritesMovedTables(t *testing.T) {
	dir := t.TempDir()

	// A flat filter-less bottom level in one key range, plus a filter-less
	// upper-level table in a disjoint range: with no bottom-level overlap the
	// latter enters the bottom level through a move compaction, which relinks
	// the file without rewriting it.
	testDB, err := New(dir)
	require.NoError(t, err)
	pDB := testDB.(*DB)
	require.NoError(t, pDB.Put([]byte{1, 1}, []byte{1}))
	require.NoError(t, pDB.Put([]byte{1, 2}, []byte{1}))
	require.NoError(t, pDB.CompactAll(t.Context(), false))
	require.NoError(t, pDB.Put([]byte{2, 2}, []byte{2}))
	require.NoError(t, pDB.Put([]byte{2, 3}, []byte{2}))
	require.NoError(t, pDB.db.Flush())
	require.NoError(t, pDB.Close())

	testDB, err = New(dir, WithBloomFilter())
	require.NoError(t, err)
	pDB = testDB.(*DB)

	require.NoError(t, pDB.CompactAll(t.Context(), true))

	tables, err := pDB.db.SSTables(pebble.WithProperties())
	require.NoError(t, err)
	for _, level := range tables {
		for i := range level {
			assert.Equal(t, "rocksdb.BuiltinBloomFilter", level[i].Properties.FilterPolicyName,
				"table %s has no filter", level[i].FileNum)
		}
	}

	for _, key := range [][]byte{{1, 1}, {1, 2}, {2, 2}, {2, 3}} {
		require.NoError(t, pDB.Get(key, func(value []byte) error {
			assert.Equal(t, key[:1], value)
			return nil
		}))
	}
	require.NoError(t, pDB.Close())
}

func TestCompactAllForceSingleKeyTable(t *testing.T) {
	dir := t.TempDir()

	// A flat filter-less bottom level holding one sstable with a single key:
	// a foreign tombstone has no room inside its range, only the same-key
	// rewrite marker can force it.
	testDB, err := New(dir)
	require.NoError(t, err)
	require.NoError(t, testDB.Put([]byte{7}, []byte{7}))
	require.NoError(t, testDB.(*DB).CompactAll(t.Context(), false))
	require.NoError(t, testDB.Close())

	testDB, err = New(dir, WithBloomFilter())
	require.NoError(t, err)
	pDB := testDB.(*DB)

	require.NoError(t, pDB.CompactAll(t.Context(), true))

	policies := bottomFilterPolicies(t, pDB)
	require.NotEmpty(t, policies)
	for _, policy := range policies {
		assert.Equal(t, "rocksdb.BuiltinBloomFilter", policy)
	}

	require.NoError(t, pDB.Get([]byte{7}, func(value []byte) error {
		assert.Equal(t, []byte{7}, value)
		return nil
	}))
	require.NoError(t, pDB.Close())
}

// bottomFilterPolicies returns the filter policy name of every non-empty
// bottom-level sstable.
func bottomFilterPolicies(t *testing.T, pDB *DB) []string {
	t.Helper()
	tables, err := pDB.db.SSTables(pebble.WithProperties())
	require.NoError(t, err)

	var policies []string
	bottom := tables[len(tables)-1]
	for i := range bottom {
		if name := bottom[i].Properties.FilterPolicyName; name != "" {
			policies = append(policies, name)
		}
	}
	return policies
}

func TestCalculatePrefixSize(t *testing.T) {
	t.Run("empty db", func(t *testing.T) {
		testDB := newPebbleMem(t)
		s, err := CalculatePrefixSize(t.Context(), testDB, []byte("0"), true)
		require.NoError(t, err)
		assert.Zero(t, s.Count)
		assert.Zero(t, s.Size)
	})

	t.Run("non empty db but empty prefix", func(t *testing.T) {
		testDB := newPebbleMem(t)
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

		testDB := newPebbleMem(t)
		require.NoError(t, testDB.Put(k1, v1))
		require.NoError(t, testDB.Put(k2, v2))
		require.NoError(t, testDB.Put(k3, v3))

		s, err := CalculatePrefixSize(t.Context(), testDB, p, true)
		require.NoError(t, err)
		assert.Equal(t, uint(3), s.Count)
		assert.Equal(t, db.DataSize(expectedSize), s.Size)

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
