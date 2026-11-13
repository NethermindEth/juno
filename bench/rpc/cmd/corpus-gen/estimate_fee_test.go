package main_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateFeeCorpus(t *testing.T) {
	node := newFakeNode(fakeInvokeTx, fakeL1HandlerTx)
	server := httptest.NewServer(node)
	defer server.Close()

	corpus := runCorpusGen(t, "estimateFee", "--count", "1", "--source-url", server.URL)

	require.Len(t, corpus.Requests, 1)
	entry := corpus.Requests[0]
	require.Equal(t, "starknet_estimateFee", entry.Method)

	var params struct {
		Request         []map[string]json.RawMessage `json:"request"`
		SimulationFlags []string                     `json:"simulation_flags"`
		BlockID         struct {
			BlockNumber uint64 `json:"block_number"`
		} `json:"block_id"`
	}
	require.NoError(t, json.Unmarshal(entry.Params, &params))

	require.Len(t, params.Request, 1, "the l1 handler ends the broadcastable prefix")
	require.NotContains(t, params.Request[0], "transaction_hash")
	require.Contains(t, params.Request[0], "signature")
	require.Equal(t, []string{}, params.SimulationFlags)

	require.Equal(t, []uint64{params.BlockID.BlockNumber + 1}, node.blocksAsked())
	require.Equal(t, []string{"INCLUDE_PROOF_FACTS"}, node.proofFactsFlags())
	require.Equal(t, 1, node.verifications(), "the sample is replayed against the source node")
}
