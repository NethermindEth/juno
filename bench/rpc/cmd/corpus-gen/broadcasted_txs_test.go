package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	invokeTx        = `{"type":"INVOKE","version":"0x3"}`
	deployAccountTx = `{"type":"DEPLOY_ACCOUNT","version":"0x3"}`
	declareTx       = `{"type":"DECLARE","version":"0x3"}`
	l1HandlerTx     = `{"type":"L1_HANDLER","version":"0x0"}`
	preV3InvokeTx   = `{"type":"INVOKE","version":"0x1"}`
)

func TestBroadcastablePrefix(t *testing.T) {
	defaultTypes := []string{invokeTxType, deployAccountTxType}

	tests := []struct {
		name    string
		txs     []string
		txTypes []string
		want    int
	}{
		{
			name:    "all broadcastable",
			txs:     []string{invokeTx, deployAccountTx},
			txTypes: defaultTypes,
			want:    2,
		},
		{
			name:    "stops at l1 handler",
			txs:     []string{invokeTx, l1HandlerTx, invokeTx},
			txTypes: defaultTypes,
			want:    1,
		},
		{
			name:    "stops at pre-v3 transaction",
			txs:     []string{invokeTx, preV3InvokeTx},
			txTypes: defaultTypes,
			want:    1,
		},
		{
			name:    "stops at unlisted declare",
			txs:     []string{invokeTx, declareTx, invokeTx},
			txTypes: defaultTypes,
			want:    1,
		},
		{
			name:    "keeps listed declare",
			txs:     []string{invokeTx, declareTx, invokeTx},
			txTypes: []string{invokeTxType, declareTxType},
			want:    3,
		},
		{
			name:    "stops at unlisted deploy account",
			txs:     []string{invokeTx, deployAccountTx},
			txTypes: []string{invokeTxType},
			want:    1,
		},
		{
			name:    "first transaction not broadcastable",
			txs:     []string{l1HandlerTx, invokeTx},
			txTypes: defaultTypes,
			want:    0,
		},
		{
			name:    "empty block",
			txs:     nil,
			txTypes: defaultTypes,
			want:    0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			txs := make([]broadcastedTx, len(test.txs))
			for i, encoded := range test.txs {
				txs[i] = txFrom(t, encoded)
			}
			require.Len(t, broadcastablePrefix(txs, test.txTypes), test.want)
		})
	}
}

func TestTxTypesRejectsUnknownType(t *testing.T) {
	err := corpusGenError(t, "estimateFee", "--tx-types", "INVOKE_FUNCTION")
	require.Contains(t, err, `--tx-types must be INVOKE, DECLARE or DEPLOY_ACCOUNT (got "INVOKE_FUNCTION")`)
}

func TestTxTypesRejectsL1Handler(t *testing.T) {
	err := corpusGenError(t, "simulateTransactions", "--tx-types", "invoke,l1_handler")
	require.Contains(t, err, "--tx-types L1_HANDLER cannot be broadcast")
}

func TestStripResponseFields(t *testing.T) {
	tx := txFrom(t, `{
		"type": "DEPLOY_ACCOUNT",
		"transaction_hash": "0x1",
		"contract_address": "0x2",
		"class_hash": "0x3"
	}`)

	stripResponseFields(tx)

	require.NotContains(t, tx, "transaction_hash")
	require.NotContains(t, tx, "contract_address")
	require.Contains(t, tx, "class_hash")
}

func txFrom(t *testing.T, encoded string) broadcastedTx {
	t.Helper()
	var tx broadcastedTx
	require.NoError(t, json.Unmarshal([]byte(encoded), &tx))
	return tx
}

func TestExecutionParamsKeepEmptyFlags(t *testing.T) {
	estimateFee, err := json.Marshal(estimateFeeParams{
		Request:         []broadcastedTx{},
		SimulationFlags: []string{},
		BlockID:         blockNumberID{7},
	})
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"request":[],"simulation_flags":[],"block_id":{"block_number":7}}`,
		string(estimateFee),
	)

	simulate, err := json.Marshal(simulateTxsParams{
		BlockID:         blockNumberID{7},
		Transactions:    []broadcastedTx{},
		SimulationFlags: []string{},
	})
	require.NoError(t, err)
	require.JSONEq(
		t,
		`{"block_id":{"block_number":7},"transactions":[],"simulation_flags":[]}`,
		string(simulate),
	)
}
