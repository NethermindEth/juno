package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

const sepoliaChainID = "0x534e5f5345504f4c4941"

// fakeNode serves a JSON-RPC source node at the given head and spec version.
func fakeNode(t *testing.T, head uint64, version string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				BlockID blockNumberID `json:"block_id"`
			} `json:"params"`
		}
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&req)) {
			return
		}
		var result any
		switch req.Method {
		case "starknet_specVersion":
			result = version
		case "starknet_chainId":
			result = sepoliaChainID
		case "starknet_blockNumber":
			result = head
		case "starknet_getBlockWithTxHashes":
			number := req.Params.BlockID.BlockNumber
			result = txHashesBlock{BlockHash: blockHash(number), Transactions: txHashes(number)}
		default:
			t.Errorf("unexpected method %s", req.Method)
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": 1, "result": result}
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// txHashes gives block n a different hash for each of its n%4 transactions,
// so blocks 0, 4, 8, ... have none.
func txHashes(number uint64) []string {
	hashes := []string{}
	for i := range number % 4 {
		hashes = append(hashes, fmt.Sprintf("0x%x", 0x2000+number*0x10+i))
	}
	return hashes
}

func blockHash(number uint64) string {
	return fmt.Sprintf("0x%x", 0x1000+number)
}

type testCorpus struct {
	Meta struct {
		ChainID  string          `json:"chainId"`
		Count    int             `json:"count"`
		Sampling blockRangeFlags `json:"sampling"`
	} `json:"meta"`
	Requests []json.RawMessage `json:"requests"`
}

type testRequest struct {
	ID     int `json:"id"`
	Params struct {
		BlockID json.RawMessage `json:"block_id"`
	} `json:"params"`
}

func runCorpusGen(t *testing.T, url string, args ...string) (testCorpus, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append(args, "--source-url", url))
	if err := cmd.Execute(); err != nil {
		return testCorpus{}, err
	}
	var c testCorpus
	require.NoError(t, json.Unmarshal(out.Bytes(), &c))
	return c, nil
}

// blockIDs decodes the block_id of each request, for a corpus without batches.
func blockIDs(t *testing.T, requests []json.RawMessage) []string {
	t.Helper()
	ids := make([]string, len(requests))
	for i, raw := range requests {
		var req testRequest
		require.NoError(t, json.Unmarshal(raw, &req))
		require.Equal(t, i+1, req.ID)
		ids[i] = string(req.Params.BlockID)
	}
	return ids
}

func TestBlocks(t *testing.T) {
	url := fakeNode(t, 100, "0.10.0")

	t.Run("samples each block once in order", func(t *testing.T) {
		c, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "7,3,9")
		require.NoError(t, err)
		assert.Equal(t, sepoliaChainID, c.Meta.ChainID)
		assert.Equal(t, 3, c.Meta.Count)
		assert.Equal(t, blockRangeFlags{Start: 3, End: 9, Blocks: []uint64{7, 3, 9}}, c.Meta.Sampling)
		assert.Equal(t, []string{
			`{"block_number":7}`, `{"block_number":3}`, `{"block_number":9}`,
		}, blockIDs(t, c.Requests))
	})

	t.Run("block hashes point to the same blocks", func(t *testing.T) {
		c, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "7,3", "--block-id", "hash")
		require.NoError(t, err)
		assert.Equal(t, []string{
			fmt.Sprintf(`{"block_hash":%q}`, blockHash(7)),
			fmt.Sprintf(`{"block_hash":%q}`, blockHash(3)),
		}, blockIDs(t, c.Requests))
	})

	t.Run("batches fill entries in block order", func(t *testing.T) {
		c, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "7,3,9", "--batch", "2")
		require.NoError(t, err)
		assert.Equal(t, 2, c.Meta.Count)
		var got [][]testRequest
		for _, raw := range c.Requests {
			var batch []testRequest
			require.NoError(t, json.Unmarshal(raw, &batch))
			got = append(got, batch)
		}
		require.Len(t, got, 2)
		var blocks []string
		for _, batch := range got {
			for _, req := range batch {
				blocks = append(blocks, fmt.Sprintf("%d:%s", req.ID, req.Params.BlockID))
			}
		}
		assert.Equal(t, []string{
			`1:{"block_number":7}`, `2:{"block_number":3}`,
			`3:{"block_number":9}`, `4:{"block_number":7}`,
		}, blocks)
	})

	t.Run("output does not depend on the head", func(t *testing.T) {
		args := []string{"getTransactionByHash", "--blocks", "7,3", "--count", "4"}
		low, err := runCorpusGen(t, fakeNode(t, 50, "0.10.0"), args...)
		require.NoError(t, err)
		high, err := runCorpusGen(t, fakeNode(t, 5000, "0.10.0"), args...)
		require.NoError(t, err)
		assert.Equal(t, low.Meta.Sampling, high.Meta.Sampling)
		assert.Equal(t, low.Requests, high.Requests)
	})

	t.Run("rejects a block after the head", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "7,101")
		require.ErrorContains(t, err, "--blocks has block 101, after the latest block (100)")
	})

	t.Run("rejects a block range too", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "7", "--block-end", "9")
		require.ErrorContains(t, err, "none of the others can be")
	})

	t.Run("events windows hold each listed block", func(t *testing.T) {
		args := []string{"getEvents", "--blocks", "2,50", "--window", "5", "--count", "6"}
		low, err := runCorpusGen(t, fakeNode(t, 54, "0.10.0"), args...)
		require.NoError(t, err)
		high, err := runCorpusGen(t, fakeNode(t, 5000, "0.10.0"), args...)
		require.NoError(t, err)
		assert.Equal(t, low.Requests, high.Requests, "the windows do not depend on the head")
		anchors := []uint64{2, 50}
		for i, raw := range low.Requests {
			var req struct {
				Params eventsParamsJSON `json:"params"`
			}
			require.NoError(t, json.Unmarshal(raw, &req))
			from, to := req.Params.Filter.FromBlock.BlockNumber, req.Params.Filter.ToBlock.BlockNumber
			anchor := anchors[i%len(anchors)]
			assert.Equal(t, uint64(4), to-from, "window size")
			assert.LessOrEqual(t, from, anchor)
			assert.GreaterOrEqual(t, to, anchor)
		}
	})

	t.Run("rejects an events window after the head", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getEvents", "--blocks", "98", "--window", "5")
		require.ErrorContains(t, err, "--window max (5) reaches block 102, after the latest block (100)")
	})

	t.Run("names a listed block with nothing to sample", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getTransactionByHash", "--blocks", "7,4")
		require.ErrorContains(t, err, "starknet_getTransactionByHash: block 4: no candidate found")
	})

	t.Run("rejects an empty list", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "")
		require.Error(t, err)
	})

	t.Run("rejects latest block ids", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getBlockWithTxHashes", "--blocks", "7", "--block-id", "latest")
		require.ErrorContains(t, err, "--block-id latest ignores --blocks")
	})

	t.Run("rejects storage proofs", func(t *testing.T) {
		_, err := runCorpusGen(t, url, "getStorageProof", "--blocks", "7")
		require.ErrorContains(t, err, "--blocks is not supported")
	})
}

type eventsParamsJSON struct {
	Filter struct {
		FromBlock blockNumberID `json:"from_block"`
		ToBlock   blockNumberID `json:"to_block"`
	} `json:"filter"`
}

// TestRangeSampling pins range-mode output, so changes for --blocks cannot
// change the corpora (and seeds) of existing range commands.
func TestRangeSampling(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"getTransactionByHash", "--block-start", "1", "--block-end", "99", "--count", "4",
		"--source-url", fakeNode(t, 100, "0.10.0"),
	})
	require.NoError(t, cmd.Execute())
	var c struct {
		Meta struct {
			Sampling json.RawMessage `json:"sampling"`
		} `json:"meta"`
		Requests []struct {
			Params txHashParams `json:"params"`
		} `json:"requests"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &c))
	assert.JSONEq(t, `{"start":1,"end":99,"responseFlags":[]}`, string(c.Meta.Sampling))
	var hashes []string
	for _, r := range c.Requests {
		hashes = append(hashes, r.Params.TransactionHash)
	}
	assert.Equal(t, []string{"0x2050", "0x25b0", "0x2561", "0x21a0"}, hashes)
}

func TestRequestFlagsNeedSpecV010(t *testing.T) {
	_, err := runCorpusGen(t, fakeNode(t, 100, "0.9.0"),
		"getBlockWithTxHashes", "--count", "1")
	require.NoError(t, err, "no request flag, so any spec version is fine")

	_, err = runCorpusGen(t, fakeNode(t, 100, "0.9.0"),
		"getBlockWithTxs", "--include-proof-facts", "--count", "1")
	require.ErrorContains(t, err,
		"--include-proof-facts needs spec v0.10 or later (source is v0.9.0)")

	_, err = runCorpusGen(t, fakeNode(t, 100, "1.0.0"),
		"getBlockWithTxs", "--include-proof-facts", "--count", "1")
	require.NoError(t, err, "a later major version has the flags")

	c, err := runCorpusGen(t, fakeNode(t, 100, "0.10.0"),
		"getBlockWithTxs", "--include-proof-facts", "--blocks", "5")
	require.NoError(t, err)
	require.Len(t, c.Requests, 1)
	assert.Contains(t, string(c.Requests[0]), `"response_flags":["INCLUDE_PROOF_FACTS"]`)
}
