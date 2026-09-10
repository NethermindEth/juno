package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimulateTransactionsCorpus(t *testing.T) {
	node := newFakeNode(fakeInvokeTx, fakeL1HandlerTx)
	server := httptest.NewServer(node)
	defer server.Close()

	corpus := runCorpusGen(
		t, "simulateTransactions", "--count", "1", "--source-url", server.URL, "--skip-fee-charge",
	)

	require.Len(t, corpus.Requests, 1)
	entry := corpus.Requests[0]
	require.Equal(t, "starknet_simulateTransactions", entry.Method)

	var params struct {
		Transactions    []map[string]json.RawMessage `json:"transactions"`
		SimulationFlags []string                     `json:"simulation_flags"`
	}
	require.NoError(t, json.Unmarshal(entry.Params, &params))

	require.Len(t, params.Transactions, 1)
	require.Equal(t, []string{skipFeeChargeFlag}, params.SimulationFlags)
	require.Equal(t, 1, node.verifications())
}
