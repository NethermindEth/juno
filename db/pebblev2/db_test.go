package pebblev2

import (
	"bytes"
	"context"
	"fmt"
	"slices"
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

func TestStaleChunksMultiLevel(t *testing.T) {
	dir := t.TempDir()
	small := func(opts *pebble.Options) error {
		opts.MemTableSize = 1 << 20
		for i := range opts.TargetFileSizes {
			opts.TargetFileSizes[i] = 256 << 10
		}
		return nil
	}

	// Fill with automatic compaction running so tables settle into several
	// levels, like on a long-running node. Wide upper-level tables straddle
	// the bottom-level ones and must not chain the chunks together.
	testDB, err := New(dir, small)
	require.NoError(t, err)
	pDB := testDB.(*DB)
	for round := range 3 {
		for done := 0; done < 80000; done += 10000 {
			batch := pDB.NewBatch()
			for i := done; i < done+10000; i++ {
				key := fmt.Appendf(nil, "key-%03d-%06d", (i*7+round)%977, i)
				require.NoError(t, batch.Put(key, bytes.Repeat([]byte{byte(i)}, 512)))
			}
			require.NoError(t, batch.Write())
		}
	}
	require.NoError(t, pDB.Close())

	testDB, err = New(dir, small, WithBloomFilter(), WithOfflineCompaction())
	require.NoError(t, err)
	pDB = testDB.(*DB)
	defer pDB.Close()

	tables, err := pDB.db.SSTables()
	require.NoError(t, err)
	var levels int
	for _, level := range tables {
		if len(level) > 0 {
			levels++
		}
	}
	require.Greater(t, levels, 1, "fill did not span multiple levels")

	baseline, err := pDB.maxTableNum()
	require.NoError(t, err)
	chunks, err := pDB.staleChunks(baseline)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1, "multi-level database must split into several chunks")

	slices.SortFunc(chunks, func(a, b forceChunk) int { return bytes.Compare(a.start, b.start) })
	for i := range chunks {
		require.Negative(t, bytes.Compare(chunks[i].start, chunks[i].end))
		if i > 0 {
			require.Negative(t, bytes.Compare(chunks[i-1].end, chunks[i].start),
				"chunks %d and %d overlap", i-1, i)
		}
	}
}

func TestCompactAllForceSkipsUpToDateTables(t *testing.T) {
	dir := t.TempDir()

	testDB, err := New(dir, WithCompression("zstd"))
	require.NoError(t, err)
	for i := range byte(8) {
		require.NoError(t, testDB.Put([]byte{i}, []byte{i}))
	}
	require.NoError(t, testDB.(*DB).CompactAll(t.Context(), false))
	require.NoError(t, testDB.Close())

	testDB, err = New(dir, WithCompression("zstd"), WithBloomFilter(), WithOfflineCompaction())
	require.NoError(t, err)
	pDB := testDB.(*DB)
	defer pDB.Close()

	tableNums := func() map[pebble.TableNum]bool {
		tables, err := pDB.db.SSTables()
		require.NoError(t, err)
		nums := make(map[pebble.TableNum]bool)
		for _, level := range tables {
			for i := range level {
				nums[level[i].FileNum] = true
			}
		}
		return nums
	}

	require.NoError(t, pDB.CompactAll(t.Context(), true))
	require.NotEmpty(t, bottomFilterPolicies(t, pDB))

	before := tableNums()
	require.NoError(t, pDB.CompactAll(t.Context(), true))
	assert.Equal(t, before, tableNums(), "second forced compaction must rewrite nothing")
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
