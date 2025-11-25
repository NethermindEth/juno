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

const (
	layerKeyCount = 50
	testPerLayer  = 5
)

type testKey struct {
	number uint64
	hash   felt.Felt
	slot   []byte
}

type testEntry struct {
	key   testKey
	value felt.Felt
}

func (k testKey) Marshal() []byte {
	return slices.Concat(
		key.Uint64.Marshal(k.number),
		key.Felt.Marshal(&k.hash),
		key.Bytes.Marshal(k.slot),
	)
}

var bucket = prefix.NewPrefixedBucket(
	typed.NewBucket(
		db.Bucket(0),
		key.Marshal[testKey](),
		value.Felt,
	),
	prefix.Prefix(
		key.Uint64,
		prefix.Prefix(
			key.Felt,
			prefix.Prefix(
				key.Bytes,
				prefix.End[testKey](),
			),
		),
	),
)

func extractExpectedEntries(
	entries map[uint64]map[felt.Felt]map[string]testEntry,
	filterBlockNumber *uint64,
	filterHash *felt.Felt,
	filterSlot *string,
) []testEntry {
	res := make([]testEntry, 0)
	for blockNumber := range filter(filterBlockNumber, cmp.Compare, entries) {
		for hash := range filter(filterHash, compareFelt, entries[blockNumber]) {
			for slot := range filter(filterSlot, cmp.Compare, entries[blockNumber][hash]) {
				res = append(res, entries[blockNumber][hash][slot])
			}
		}
	}
	return res
}

func compareFelt(a, b felt.Felt) int {
	return a.Cmp(&b)
}

func filter[K comparable, V any](filter *K, cmp func(K, K) int, entries map[K]V) iter.Seq[K] {
	if filter != nil {
		return func(yield func(K) bool) {
			yield(*filter)
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

func validateResult(
	t *testing.T,
	expected []testEntry,
	actual iter.Seq2[prefix.Entry[felt.Felt], error],
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

func TestPrefixedBucket(t *testing.T) {
	database := memory.New()
	content := make(map[uint64]map[felt.Felt]map[string]testEntry)

	t.Run("Populate database", func(t *testing.T) {
		for range layerKeyCount {
			blockNumber := rand.Uint64()
			content[blockNumber] = make(map[felt.Felt]map[string]testEntry)

			for range layerKeyCount {
				hash := felt.Random[felt.Felt]()
				content[blockNumber][hash] = make(map[string]testEntry)

				for range layerKeyCount {
					slot := cryptorand.Text()
					value := felt.Random[felt.Felt]()

					key := testKey{
						number: blockNumber,
						hash:   hash,
						slot:   []byte(slot),
					}
					entry := testEntry{
						key:   key,
						value: value,
					}

					content[blockNumber][hash][slot] = entry
					require.NoError(t, bucket.Put(database, key, &value))
				}
			}
		}
	})

	t.Run("Full scan", func(t *testing.T) {
		validateResult(
			t,
			extractExpectedEntries(content, nil, nil, nil),
			bucket.Scan(database, bucket.Prefix().Finish()),
		)
	})

	t.Run("1 layer scan", func(t *testing.T) {
		for range testPerLayer {
			blockNumber, _ := randomEntry(t, content)
			validateResult(
				t,
				extractExpectedEntries(content, &blockNumber, nil, nil),
				bucket.Scan(database, bucket.Prefix().Add(blockNumber).Finish()),
			)
		}
	})

	t.Run("2 layer scan", func(t *testing.T) {
		for range testPerLayer * testPerLayer {
			blockNumber, map1 := randomEntry(t, content)
			hash, _ := randomEntry(t, map1)
			validateResult(
				t,
				extractExpectedEntries(content, &blockNumber, &hash, nil),
				bucket.Scan(database, bucket.Prefix().Add(blockNumber).Add(&hash).Finish()),
			)
		}
	})

	t.Run("3 layer scan", func(t *testing.T) {
		for range testPerLayer * testPerLayer * testPerLayer {
			blockNumber, map1 := randomEntry(t, content)
			hash, map2 := randomEntry(t, map1)
			slot, _ := randomEntry(t, map2)
			validateResult(
				t,
				extractExpectedEntries(content, &blockNumber, &hash, &slot),
				bucket.Scan(database, bucket.Prefix().Add(blockNumber).Add(&hash).Add([]byte(slot))),
			)
		}
	})
}
