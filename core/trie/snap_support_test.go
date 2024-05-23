package trie_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trieHeight = 251

func TestRangeAndVerify(t *testing.T) {
	scenarios := []struct {
		name             string
		startQuery       *felt.Felt
		limitQuery       *felt.Felt
		maxNode          int
		expectedKeyCount int
		hasMore          bool
		noProof          bool
	}{
		{
			name:             "all",
			startQuery:       numToFelt(0),
			expectedKeyCount: 10,
			hasMore:          false,
		},
		{
			name:             "all without start query",
			expectedKeyCount: 10,
			hasMore:          false,
			noProof:          true,
		},
		{
			name:             "start in the middle",
			startQuery:       numToFelt(500),
			expectedKeyCount: 5,
			hasMore:          false,
		},
		{
			name:             "start with limit query",
			startQuery:       numToFelt(100),
			limitQuery:       numToFelt(500),
			expectedKeyCount: 5,
			hasMore:          true,
		},
		{
			name:             "start with limit query and node count limit",
			startQuery:       numToFelt(100),
			limitQuery:       numToFelt(500),
			maxNode:          3,
			expectedKeyCount: 3,
			hasMore:          true,
		},
		{
			name:             "finished before limit query",
			startQuery:       numToFelt(100),
			limitQuery:       numToFelt(20000),
			expectedKeyCount: 9,
			hasMore:          false,
		},
		{
			name:             "last one right after limit query",
			startQuery:       numToFelt(100),
			limitQuery:       numToFelt(900),
			expectedKeyCount: 9,
			hasMore:          false,
		},
		{
			name:             "two leaf after limit query, last leaf skipped",
			startQuery:       numToFelt(100),
			limitQuery:       numToFelt(800),
			expectedKeyCount: 8,
			hasMore:          true,
		},
		{
			name:             "no node between start and limit",
			startQuery:       numToFelt(450),
			limitQuery:       numToFelt(451),
			expectedKeyCount: 1,
			hasMore:          true,
		},
		{
			name:             "start query after last node",
			startQuery:       numToFelt(10000),
			expectedKeyCount: 0,
			hasMore:          false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			storage := trie.NewStorage(db.NewMemTransaction(), []byte{})
			testTrie, err := trie.NewTriePedersen(storage, 251)
			assert.NoError(t, err)

			for i := 0; i < 10; i++ {
				_, err = testTrie.Put(numToFelt(i*100+1), numToFelt(i*100+2))
				assert.NoError(t, err)
			}

			expectedRoot, err := testTrie.Root()
			assert.NoError(t, err)

			startQuery := scenario.startQuery
			var keys []*felt.Felt
			var values []*felt.Felt

			proofs, _, err := testTrie.IterateAndGenerateProof(startQuery, func(key, value *felt.Felt) (bool, error) {
				keys = append(keys, key)
				values = append(values, value)
				if scenario.maxNode > 0 && len(keys) >= scenario.maxNode {
					return false, nil
				}
				if scenario.limitQuery != nil && key.Cmp(scenario.limitQuery) > 0 {
					// Last (one after limit) is included.
					return false, nil
				}
				return true, nil
			})
			assert.NoError(t, err)

			assert.Equal(t, scenario.expectedKeyCount, len(keys))
			if scenario.noProof {
				assert.Empty(t, proofs)
			}

			hasMore, valid, err := trie.VerifyRange(expectedRoot, startQuery, keys, values, proofs, crypto.Pedersen, trieHeight)
			assert.NoError(t, err)
			assert.True(t, valid)

			assert.Equal(t, scenario.hasMore, hasMore)
		})
	}
}

func TestRangeAndVerifyReject(t *testing.T) {
	scenarios := []struct {
		name       string
		startQuery *felt.Felt
		skip       bool
		maxNode    int
		mutator    func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode)
	}{
		{
			name:       "missing proofs",
			startQuery: numToFelt(500),
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				return keys, values, nil
			},
		},
		{
			name: "missing leaf when all node requested",
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				return keys[1:], values[1:], nil
			},
		},
		{
			skip:       true,
			name:       "missing part of keys at start",
			startQuery: numToFelt(500),
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				return keys[1:], values[1:], proofs
			},
		},
		{
			skip:       true,
			name:       "missing part of keys at end",
			startQuery: numToFelt(500),
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				return keys[:len(keys)-1], values[:len(keys)-1], proofs
			},
		},
		{
			skip:       true,
			name:       "missing part of keys in the middle",
			startQuery: numToFelt(500),
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				newkeys := []*felt.Felt{}
				newvalues := []*felt.Felt{}
				newkeys = append(newkeys, keys[:2]...)
				newvalues = append(newvalues, values[:2]...)
				newkeys = append(newkeys, keys[3:]...)
				newvalues = append(newvalues, values[3:]...)

				return newkeys, newvalues, proofs
			},
		},
		{
			name: "missing part of keys in the middle when whole trie is sent",
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				newkeys := []*felt.Felt{}
				newvalues := []*felt.Felt{}
				newkeys = append(newkeys, keys[:2]...)
				newvalues = append(newvalues, values[:2]...)
				newkeys = append(newkeys, keys[3:]...)
				newvalues = append(newvalues, values[3:]...)

				return newkeys, newvalues, proofs
			},
		},
		{
			name: "value changed",
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				values[3] = numToFelt(10000)
				return keys, values, proofs
			},
		},
		{
			skip:       true,
			startQuery: numToFelt(500),
			name:       "value changed when whole trie is sent",
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				values[3] = numToFelt(10000)
				return keys, values, proofs
			},
		},
		{
			name: "key changed",
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				keys[3] = numToFelt(10000)
				return keys, values, proofs
			},
		},
		{
			skip:       true,
			startQuery: numToFelt(500),
			name:       "key changed when whole trie is sent",
			mutator: func(keys, values []*felt.Felt, proofs []trie.ProofNode) ([]*felt.Felt, []*felt.Felt, []trie.ProofNode) {
				keys[3] = numToFelt(10000)
				return keys, values, proofs
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			if scenario.skip {
				t.Skip()
			}

			storage := trie.NewStorage(db.NewMemTransaction(), []byte{})
			testTrie, err := trie.NewTriePedersen(storage, 251)
			assert.NoError(t, err)

			for i := 0; i < 10; i++ {
				_, err = testTrie.Put(numToFelt(i*100+1), numToFelt(i*100+2))
				assert.NoError(t, err)
			}

			expectedRoot, err := testTrie.Root()
			assert.NoError(t, err)

			startQuery := scenario.startQuery
			var keys []*felt.Felt
			var values []*felt.Felt

			proofs, _, err := testTrie.IterateAndGenerateProof(startQuery, func(key, value *felt.Felt) (bool, error) {
				keys = append(keys, key)
				values = append(values, value)
				if scenario.maxNode > 0 && len(keys) >= scenario.maxNode {
					return false, nil
				}
				return true, nil
			})
			assert.NoError(t, err)

			keys, values, proofs = scenario.mutator(keys, values, proofs)

			_, valid, err := trie.VerifyRange(expectedRoot, startQuery, keys, values, proofs, crypto.Pedersen, trieHeight)
			assert.NoError(t, err)
			assert.False(t, valid)
		})
	}
}

func TestIterateOverTrie(t *testing.T) {
	memdb := pebble.NewMemTest(t)
	txn, err := memdb.NewTransaction(true)
	require.NoError(t, err)
	logger := utils.NewNopZapLogger()

	tempTrie, err := trie.NewTriePedersen(trie.NewStorage(txn, []byte{0}), 251)
	require.NoError(t, err)

	// key ranges
	var (
		bigPrefix      = uint64(1000 * 1000 * 1000 * 1000)
		count          = 100
		ranges         = 5
		fstInt, lstInt uint64
		fstKey, lstKey *felt.Felt
	)
	for i := range ranges {
		for j := range count {
			lstInt = bigPrefix*uint64(i) + uint64(count+j)
			lstKey = new(felt.Felt).SetUint64(lstInt)
			value := new(felt.Felt).SetUint64(uint64(10*count + j + i))

			if fstKey == nil {
				fstKey = lstKey
				fstInt = lstInt
			}

			_, err := tempTrie.Put(lstKey, value)
			require.NoError(t, err)
		}
	}

	maxNodes := uint32(ranges*count + 1)
	startZero := felt.Zero.Clone()

	visitor := func(start, limit *felt.Felt, max uint32) (int, bool, *felt.Felt, *felt.Felt) {
		visited := 0
		var fst, lst *felt.Felt
		_, finish, err := tempTrie.IterateWithLimit(
			start,
			limit,
			max,
			logger,
			func(key, value *felt.Felt) error {
				if fst == nil {
					fst = key
				}
				lst = key
				visited++
				return nil
			})
		require.NoError(t, err)
		return visited, finish, fst, lst
	}

	t.Run("iterate without limit", func(t *testing.T) {
		expectedLeaves := ranges * count
		visited, finish, fst, lst := visitor(nil, nil, maxNodes)
		require.Equal(t, expectedLeaves, visited)
		require.True(t, finish)
		require.Equal(t, fstKey, fst)
		require.Equal(t, lstKey, lst)
		fmt.Println("Visited:", visited, "\tFinish:", finish, "\tRange:", fst.Uint64(), "-", lst.Uint64())
	})

	t.Run("iterate over trie im chunks", func(t *testing.T) {
		chunkSize := 77
		lstChunkSize := int(math.Mod(float64(ranges*count), float64(chunkSize)))
		startKey := startZero
		for {
			visited, finish, fst, lst := visitor(startKey, nil, uint32(chunkSize))
			fmt.Println("Finish:", finish, "\tstart:", startKey.Uint64(), "\trange:", fst.Uint64(), "-", lst.Uint64())
			if finish {
				require.Equal(t, lstChunkSize, visited)
				break
			}
			require.Equal(t, chunkSize, visited)
			require.False(t, finish)
			startKey = new(felt.Felt).SetUint64(lst.Uint64() + 1)
		}
	})

	t.Run("iterate over trie im groups", func(t *testing.T) {
		startKey := startZero
		for {
			visited, finish, fst, lst := visitor(startKey, nil, uint32(count))
			if finish {
				require.Equal(t, 0, visited)
				fmt.Println("Finish:", finish, "\tstart:", startKey.Uint64(), "\trange: <empty>")
				break
			}
			fmt.Println("Finish:", finish, "\tstart:", startKey.Uint64(), "\trange:", fst.Uint64(), "-", lst.Uint64())
			require.Equal(t, count, visited)
			require.False(t, finish)
			if lst != nil {
				startKey = new(felt.Felt).SetUint64(lst.Uint64() + 1)
			}
		}
	})

	t.Run("stop before first key", func(t *testing.T) {
		lowerBound := new(felt.Felt).SetUint64(fstInt - 1)
		visited, finish, _, _ := visitor(startZero, lowerBound, maxNodes)
		require.True(t, finish)
		require.Equal(t, 0, visited)
	})

	t.Run("first key is a limit", func(t *testing.T) {
		visited, finish, fst, lst := visitor(startZero, fstKey, maxNodes)
		require.Equal(t, 1, visited)
		require.True(t, finish)
		require.Equal(t, fstKey, fst)
		require.Equal(t, fstKey, lst)
	})

	t.Run("start is the last key", func(t *testing.T) {
		visited, finish, fst, lst := visitor(lstKey, nil, maxNodes)
		require.Equal(t, 1, visited)
		require.True(t, finish)
		require.Equal(t, lstKey, fst)
		require.Equal(t, lstKey, lst)
	})

	t.Run("start and limit are the last key", func(t *testing.T) {
		visited, finish, fst, lst := visitor(lstKey, lstKey, maxNodes)
		require.Equal(t, 1, visited)
		require.True(t, finish)
		require.Equal(t, lstKey, fst)
		require.Equal(t, lstKey, lst)
	})

	t.Run("iterate after last key yields no key", func(t *testing.T) {
		upperBound := new(felt.Felt).SetUint64(lstInt + 1)
		visited, finish, fst, _ := visitor(upperBound, nil, maxNodes)
		require.Equal(t, 0, visited)
		require.True(t, finish)
		require.Nil(t, fst)
	})

	t.Run("iterate with reversed bounds yields no key", func(t *testing.T) {
		visited, finish, fst, _ := visitor(lstKey, fstKey, maxNodes)
		require.Equal(t, 0, visited)
		require.True(t, finish)
		require.Nil(t, fst)
	})

	t.Run("iterate over the first group", func(t *testing.T) {
		fstGrpBound := new(felt.Felt).SetUint64(fstInt + uint64(count-1))
		visited, finish, fst, lst := visitor(fstKey, fstGrpBound, maxNodes)
		require.Equal(t, count, visited)
		require.True(t, finish)
		require.Equal(t, fstKey, fst)
		require.Equal(t, fstGrpBound, lst)
	})

	t.Run("iterate over the first group no lower bound", func(t *testing.T) {
		fstGrpBound := new(felt.Felt).SetUint64(fstInt + uint64(count-1))
		visited, finish, fst, lst := visitor(nil, fstGrpBound, maxNodes)
		require.Equal(t, count, visited)
		require.True(t, finish)
		require.Equal(t, fstKey, fst)
		require.Equal(t, fstGrpBound, lst)
	})

	t.Run("iterate over the first group by max nodes", func(t *testing.T) {
		fstGrpBound := new(felt.Felt).SetUint64(fstInt + uint64(count-1))
		visited, finish, fst, lst := visitor(fstKey, nil, uint32(count))
		require.Equal(t, count, visited)
		require.False(t, finish)
		require.Equal(t, fstKey, fst)
		require.Equal(t, fstGrpBound, lst)
	})

	t.Run("iterate over the last group, start before group bound", func(t *testing.T) {
		lstGrpStartInt := lstInt - uint64(count-1)
		lstGrpFstKey := new(felt.Felt).SetUint64(lstGrpStartInt)
		startKey := new(felt.Felt).SetUint64(lstGrpStartInt - uint64(count))

		visited, finish, fst, lst := visitor(startKey, nil, maxNodes)
		require.Equal(t, count, visited)
		require.True(t, finish)
		require.Equal(t, lstGrpFstKey, fst)
		require.Equal(t, lstKey, lst)
	})

	sndGrpFstKey := new(felt.Felt).SetUint64(bigPrefix + uint64(count))
	sndGrpLstKey := new(felt.Felt).SetUint64(bigPrefix + uint64(2*count-1))
	t.Run("second group key selection", func(t *testing.T) {
		visited, _, _, lst := visitor(fstKey, nil, uint32(count+1))
		require.Equal(t, count+1, visited)
		require.Equal(t, sndGrpFstKey, lst)

		visited, finish, fst, lst := visitor(sndGrpFstKey, sndGrpLstKey, maxNodes)
		require.Equal(t, count, visited)
		require.True(t, finish)
		require.Equal(t, sndGrpFstKey, fst)
		require.Equal(t, sndGrpLstKey, lst)
	})

	t.Run("second group key selection 2", func(t *testing.T) {
		nodeAfterFstGrp := new(felt.Felt).SetUint64(fstInt + uint64(count+1))
		visited, _, fst, lst := visitor(nodeAfterFstGrp, nil, 1)
		require.Equal(t, 1, visited)
		require.Equal(t, sndGrpFstKey, fst)
		require.Equal(t, fst, lst)

		visited, finish, fst, lst := visitor(sndGrpFstKey, nil, uint32(count))
		require.Equal(t, count, visited)
		require.False(t, finish)
		require.Equal(t, sndGrpFstKey, fst)
		require.Equal(t, sndGrpLstKey, lst)
	})
}
