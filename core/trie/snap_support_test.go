package trie_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

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
				_, err := testTrie.Put(numToFelt(i*100+1), numToFelt(i*100+2))
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

			hasMore, valid, err := trie.VerifyRange(expectedRoot, startQuery, keys, values, proofs, crypto.Pedersen)
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
				_, err := testTrie.Put(numToFelt(i*100+1), numToFelt(i*100+2))
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

			_, valid, err := trie.VerifyRange(expectedRoot, startQuery, keys, values, proofs, crypto.Pedersen)
			assert.NoError(t, err)
			assert.False(t, valid)
		})
	}
}
