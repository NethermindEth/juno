package main

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
	require.Equal(t, []string{includeProofFactsFlag}, node.proofFactsFlags())
	require.Equal(t, 1, node.verifications(), "the sample is replayed against the source node")
}

func TestEstimateFeeKeepsListedDeclare(t *testing.T) {
	node := newFakeNode(fakeDeclareTx, fakeInvokeTx)
	server := httptest.NewServer(node)
	defer server.Close()

	corpus := runCorpusGen(
		t, "estimateFee", "--count", "1", "--tx-types", "invoke,declare", "--source-url", server.URL,
	)

	require.Len(t, corpus.Requests, 1)
	var params struct {
		Request []map[string]json.RawMessage `json:"request"`
	}
	require.NoError(t, json.Unmarshal(corpus.Requests[0].Params, &params))

	var sampling struct {
		TxTypes []string `json:"txTypes"`
	}
	require.NoError(t, json.Unmarshal(corpus.Meta.Sampling, &sampling))
	require.Equal(t, []string{invokeTxType, declareTxType}, sampling.TxTypes)

	require.Len(t, params.Request, 1)
	declare := params.Request[0]
	require.JSONEq(t, fakeContractClass, string(declare["contract_class"]))
	require.NotContains(t, declare, "class_hash")
	require.Contains(t, declare, "compiled_class_hash")
	require.Equal(t, []string{"0x9"}, node.classesAsked())
}

func TestEstimateFeeStopsAtUnlistedType(t *testing.T) {
	node := newFakeNode(fakeDeclareTx, fakeInvokeTx)
	server := httptest.NewServer(node)
	defer server.Close()

	err := corpusGenError(t, "estimateFee", "--count", "1", "--source-url", server.URL)

	require.Contains(t, err, "is 0 long, want 1")
	require.Empty(t, node.classesAsked())
}
