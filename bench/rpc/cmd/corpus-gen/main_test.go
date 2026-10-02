package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCorpusSeed(t *testing.T) {
	const method = "starknet_getBlockWithTxs"
	base := blockIDWithProofFactsArgs{
		blockIDArgs: blockIDArgs{
			blockRangeFlags: blockRangeFlags{Start: 0, End: 100},
			BlockIDKind:     blockIDNumber,
		},
		ResponseFlags: txnFlags{},
	}
	baseSeed, err := corpusSeed(1, method, base)
	require.NoError(t, err)

	t.Run("same options reproduce the seed", func(t *testing.T) {
		again, err := corpusSeed(1, method, base)
		require.NoError(t, err)
		require.Equal(t, baseSeed, again)
	})

	hashKind := base
	hashKind.BlockIDKind = blockIDHash
	proofFacts := base
	proofFacts.ResponseFlags = txnFlags{includeProofFactsFlag}
	tests := map[string]struct {
		seed     uint64
		method   string
		sampling any
	}{
		"user seed":      {seed: 2, method: method, sampling: base},
		"method":         {seed: 1, method: "starknet_getBlockWithReceipts", sampling: base},
		"block id kind":  {seed: 1, method: method, sampling: hashKind},
		"response flags": {seed: 1, method: method, sampling: proofFacts},
	}
	for name, tt := range tests {
		t.Run(name+" changes the seed", func(t *testing.T) {
			got, err := corpusSeed(tt.seed, tt.method, tt.sampling)
			require.NoError(t, err)
			require.NotEqual(t, baseSeed, got)
		})
	}
}
