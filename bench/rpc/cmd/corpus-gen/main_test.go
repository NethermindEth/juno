package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestCorpusMetaChainID(t *testing.T) {
	const sepolia = "0x534e5f5345504f4c4941"
	tests := map[string]struct {
		args         []string
		chainIDError bool
		wantErr      string
	}{
		"sampled method":  {args: []string{"getBlockWithTxHashes", "--count", "2", "--block-end", "5"}},
		"no-param method": {args: []string{"blockNumber", "--count", "1"}},
		"chain id error": {
			args:         []string{"blockNumber", "--count", "1"},
			chainIDError: true,
			wantErr:      "fetch chain id",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req jsonRPCRequest
				if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
					return
				}
				resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
				switch req.Method {
				case "starknet_chainId":
					if tt.chainIDError {
						resp["error"] = rpcError{Code: -32603, Message: "Internal error"}
					} else {
						resp["result"] = sepolia
					}
				case "starknet_specVersion":
					resp["result"] = "0.10.3"
				case "starknet_blockNumber":
					resp["result"] = 10
				case "starknet_getBlockWithTxHashes":
					resp["result"] = txHashesBlock{BlockHash: "0x1"}
				}
				assert.NoError(t, json.NewEncoder(w).Encode(resp))
			}))
			t.Cleanup(node.Close)

			cmd := newRootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append(tt.args, "--source-url", node.URL))
			err := cmd.Execute()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			var c struct {
				Meta struct {
					ChainID string `json:"chainId"`
				} `json:"meta"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &c))
			assert.Equal(t, sepolia, c.Meta.ChainID)
		})
	}
}
