package prefix_test

import (
	"cmp"
	cryptorand "crypto/rand"
	"iter"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/stretchr/testify/require"
)

type entryMap = map[uint64]map[felt.Address]map[string]testEntry

const (
	layerKeyCount     = 20
	readTestsPerLayer = 5
	deleteTests       = 5
)

type testKey struct {
	number  uint64
	address felt.Address
	slot    []byte
}

type testEntry struct {
	key   testKey
	value felt.ClassHash
}

func (k testKey) Marshal() []byte {
	return slices.Concat(
		key.Uint64.Marshal(k.number),
		key.Address.Marshal(&k.address),
		key.Bytes.Marshal(k.slot),
	)
}

var bucket = prefix.NewPrefixedBucket(
	typed.NewBucket(
		db.Bucket(0),
		key.Marshal[testKey](),
		value.ClassHash,
	),
	prefix.Prefix(
		key.Uint64,
		prefix.Prefix(
			key.Address,
			prefix.Prefix(
				key.Bytes,
				prefix.End[felt.ClassHash](),
			),
		),
	),
)

func extractExpectedEntries(
	entries entryMap,
	filterBlockNumber *uint64,
	filterAddress *felt.Address,
	filterSlot *string,
) []testEntry {
	res := make([]testEntry, 0)
	for blockNumber := range filter(filterBlockNumber, cmp.Compare, entries) {
		if len(entries[blockNumber]) == 0 {
			delete(entries, blockNumber)
			continue
		}

		for address := range filter(filterAddress, compareAddress, entries[blockNumber]) {
			if len(entries[blockNumber][address]) == 0 {
				delete(entries[blockNumber], address)
				continue
			}

			for slot := range filter(filterSlot, cmp.Compare, entries[blockNumber][address]) {
				res = append(res, entries[blockNumber][address][slot])
			}
		}
	}
	return res
}

func compareAddress(a, b felt.Address) int {
	return (*felt.Felt)(&a).Cmp((*felt.Felt)(&b))
}

func filter[K comparable, V any](filter *K, cmp func(K, K) int, entries map[K]V) iter.Seq[K] {
	if filter != nil {
		return func(yield func(K) bool) {
			if _, exists := entries[*filter]; exists {
				yield(*filter)
			}
		}
	}

	return func(yield func(K) bool) {
		for _, key := range slices.SortedFunc(maps.Keys(entries), cmp) {
			if !yield(key) {
				return
			}
		}
	}
}

func randomEntry[K comparable, V any](t *testing.T, entries map[K]V) (K, V) {
	t.Helper()
	for key, value := range entries {
		return key, value
	}
	require.FailNow(t, "no entries")
	return *new(K), *new(V)
}

func randomDeletedKey[K comparable, V any](t *testing.T, entries map[K]V) K {
	t.Helper()
	key, _ := randomEntry(t, entries)
	delete(entries, key)
	return key
}

func randomDeletedRange[K comparable, V any](
	t *testing.T,
	entries map[K]V,
	cmp func(K, K) int,
) (lowerBound, upperBound K) {
	t.Helper()
	var keys []K
	for {
		lowerBound, _ = randomEntry(t, entries)
		upperBound, _ = randomEntry(t, entries)
		if cmp(lowerBound, upperBound) > 0 {
			lowerBound, upperBound = upperBound, lowerBound
		}
		keys = keys[:0]
		for key := range entries {
			if cmp(lowerBound, key) <= 0 && cmp(key, upperBound) < 0 {
				keys = append(keys, key)
			}
		}
		maxDeletions := layerKeyCount / deleteTests // delete at most half of the entries
		if len(keys) > 0 && len(keys) <= maxDeletions {
			break
		}
	}

	for _, key := range keys {
		delete(entries, key)
	}
	return
}

func validateResult(
	t *testing.T,
	expected []testEntry,
	actual iter.Seq2[prefix.Entry[felt.ClassHash], error],
) {
	t.Helper()
	count := 0
	for entry, err := range actual {
		require.NoError(t, err)
		require.Greater(t, len(expected), count)
		require.Equal(t, bucket.Key(expected[count].key.Marshal()), entry.Key)
		require.Equal(t, expected[count].value, entry.Value)
		count++
	}
	require.Equal(t, len(expected), count)
}

func testFullScan(
	t *testing.T,
	database db.KeyValueReader,
	content entryMap,
) {
	t.Helper()
	validateResult(
		t,
		extractExpectedEntries(content, nil, nil, nil),
		bucket.Prefix().Scan(database),
	)
}

func test1LayerScan(
	t *testing.T,
	database db.KeyValueReader,
	content entryMap,
	blockNumber uint64,
) {
	t.Helper()
	validateResult(
		t,
		extractExpectedEntries(content, &blockNumber, nil, nil),
		bucket.Prefix().Add(blockNumber).Scan(database),
	)
}

func test2LayerScan(
	t *testing.T,
	database db.KeyValueReader,
	content entryMap,
	blockNumber uint64,
	address felt.Address,
) {
	t.Helper()
	validateResult(
		t,
		extractExpectedEntries(content, &blockNumber, &address, nil),
		bucket.Prefix().Add(blockNumber).Add(&address).Scan(database),
	)
}

func test3LayerScan(
	t *testing.T,
	database db.KeyValueReader,
	content entryMap,
	blockNumber uint64,
	address felt.Address,
	slot string,
) {
	t.Helper()
	validateResult(
		t,
		extractExpectedEntries(content, &blockNumber, &address, &slot),
		bucket.Prefix().Add(blockNumber).Add(&address).Add([]byte(slot)).Scan(database),
	)
}

func runReadTests(
	t *testing.T,
	database db.KeyValueReader,
	content entryMap,
) {
	t.Helper()

	t.Run("Full scan", func(t *testing.T) {
		testFullScan(t, database, content)
	})

	t.Run("1 layer scan", func(t *testing.T) {
		for range readTestsPerLayer {
			blockNumber, _ := randomEntry(t, content)
			test1LayerScan(t, database, content, blockNumber)
		}
	})

	t.Run("2 layer scan", func(t *testing.T) {
		for range readTestsPerLayer * readTestsPerLayer {
			blockNumber, map1 := randomEntry(t, content)
			hash, _ := randomEntry(t, map1)
			test2LayerScan(t, database, content, blockNumber, hash)
		}
	})

	t.Run("3 layer scan", func(t *testing.T) {
		for range readTestsPerLayer * readTestsPerLayer * readTestsPerLayer {
			blockNumber, map1 := randomEntry(t, content)
			hash, map2 := randomEntry(t, map1)
			slot, _ := randomEntry(t, map2)
			test3LayerScan(t, database, content, blockNumber, hash, slot)
		}
	})
}

func TestPrefixedBucket(t *testing.T) {
	database := memory.New()
	content := make(entryMap)

	t.Run("Populate database", func(t *testing.T) {
		for range layerKeyCount {
			blockNumber := rand.Uint64()
			content[blockNumber] = make(map[felt.Address]map[string]testEntry)

			for range layerKeyCount {
				address := felt.Random[felt.Address]()
				content[blockNumber][address] = make(map[string]testEntry)

				for range layerKeyCount {
					slot := cryptorand.Text()
					value := felt.Random[felt.ClassHash]()

					key := testKey{
						number:  blockNumber,
						address: address,
						slot:    []byte(slot),
					}
					entry := testEntry{
						key:   key,
						value: value,
					}

					content[blockNumber][address][slot] = entry
					require.NoError(t, bucket.Put(database, key, &value))
				}
			}
		}
	})

	t.Run("Scan", func(t *testing.T) {
		runReadTests(t, database, content)
	})

	t.Run("DeletePrefix", func(t *testing.T) {
		t.Run("Layer 3", func(t *testing.T) {
			for range deleteTests {
				blockNumber, map1 := randomEntry(t, content)
				hash, map2 := randomEntry(t, map1)
				slot := randomDeletedKey(t, map2)
				require.NoError(
					t,
					bucket.Prefix().Add(blockNumber).Add(&hash).Add([]byte(slot)).DeletePrefix(
						database,
					),
				)
				test3LayerScan(t, database, content, blockNumber, hash, slot)
				test2LayerScan(t, database, content, blockNumber, hash)
				test1LayerScan(t, database, content, blockNumber)
				testFullScan(t, database, content)
			}
		})

		t.Run("Layer 2", func(t *testing.T) {
			for range deleteTests {
				blockNumber, map1 := randomEntry(t, content)
				hash := randomDeletedKey(t, map1)
				require.NoError(
					t,
					bucket.Prefix().Add(blockNumber).Add(&hash).DeletePrefix(database),
				)
				test2LayerScan(t, database, content, blockNumber, hash)
				test1LayerScan(t, database, content, blockNumber)
				testFullScan(t, database, content)
			}
		})

		t.Run("Layer 1", func(t *testing.T) {
			for range deleteTests {
				blockNumber := randomDeletedKey(t, content)
				require.NoError(t, bucket.Prefix().Add(blockNumber).DeletePrefix(database))
				test1LayerScan(t, database, content, blockNumber)
				testFullScan(t, database, content)
			}
		})

		t.Run("Run full scan after delete", func(t *testing.T) {
			testFullScan(t, database, content)
		})
	})

	t.Run("DeleteRange", func(t *testing.T) {
		t.Run("Layer 3", func(t *testing.T) {
			for range deleteTests {
				blockNumber, map1 := randomEntry(t, content)
				hash, map2 := randomEntry(t, map1)
				lowerBound, upperBound := randomDeletedRange(t, map2, cmp.Compare)
				require.NoError(
					t,
					bucket.Prefix().Add(blockNumber).Add(&hash).DeleteRange(
						database,
						[]byte(lowerBound),
						[]byte(upperBound),
					),
				)
				test3LayerScan(t, database, content, blockNumber, hash, lowerBound)
				test3LayerScan(t, database, content, blockNumber, hash, upperBound)
				test2LayerScan(t, database, content, blockNumber, hash)
				test1LayerScan(t, database, content, blockNumber)
				testFullScan(t, database, content)
			}
		})

		t.Run("Layer 2", func(t *testing.T) {
			for range deleteTests {
				blockNumber, map1 := randomEntry(t, content)
				lowerBound, upperBound := randomDeletedRange(t, map1, compareAddress)
				require.NoError(t, bucket.Prefix().Add(blockNumber).DeleteRange(
					database,
					&lowerBound,
					&upperBound,
				))
				test2LayerScan(t, database, content, blockNumber, lowerBound)
				test2LayerScan(t, database, content, blockNumber, upperBound)
				test1LayerScan(t, database, content, blockNumber)
				testFullScan(t, database, content)
			}
		})

		t.Run("Layer 1", func(t *testing.T) {
			for range deleteTests {
				lowerBound, upperBound := randomDeletedRange(t, content, cmp.Compare)
				require.NoError(t, bucket.Prefix().DeleteRange(
					database,
					lowerBound,
					upperBound,
				))
				test1LayerScan(t, database, content, lowerBound)
				test1LayerScan(t, database, content, upperBound)
				testFullScan(t, database, content)
			}
		})

		t.Run("Run full scan after delete", func(t *testing.T) {
			testFullScan(t, database, content)
		})
	})
}
