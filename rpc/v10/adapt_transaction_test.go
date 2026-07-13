package rpcv10_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:lll // File paths
const (
	sepoliaInvokeTxnPath        = "../../clients/feeder/testdata/sepolia/transaction/0x76b52e17bc09064bd986ead34263e6305ef3cecfb3ae9e19b86bf4f1a1a20ea.json"
	sepoliaDeclareTxnPath       = "../../clients/feeder/testdata/sepolia/transaction/0x30c852c522274765e1d681bc8a84ce7c41118370ef2ba7d18a427ed29f5b155.json"
	sepoliaDeployAccountTxnPath = "../../clients/feeder/testdata/sepolia/transaction/0x32413f8cee053089d6d7026a72e4108262ca3cfe868dd9159bc1dd160aec975.json"
)

// loadTx reads a feeder testdata transaction file
func loadTx(t *testing.T, path string) (*rpc.BroadcastedTransaction, core.Transaction) {
	t.Helper()

	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	data, err := os.ReadFile(abs)
	require.NoError(t, err)

	var temp struct {
		Transaction starknet.Transaction `json:"transaction"`
	}
	require.NoError(t, json.Unmarshal(data, &temp))

	coreTx, err := sn2core.AdaptTransaction(&temp.Transaction)
	require.NoError(t, err)

	return &rpc.BroadcastedTransaction{
		Transaction: *rpc.AdaptCoreTransaction(coreTx),
	}, coreTx
}

func marshalAndCompare(t *testing.T, originalCoreTxn, adaptedCoreTx core.Transaction) {
	originalTxJSON, err := json.Marshal(originalCoreTxn)
	require.NoError(t, err)
	adaptedTxJSON, err := json.Marshal(adaptedCoreTx)
	require.NoError(t, err)
	assert.JSONEq(t, string(originalTxJSON), string(adaptedTxJSON))
}

func TestAdaptBroadcastedTransactionToCore(t *testing.T) {
	t.Parallel()
	network := &networks.Sepolia

	t.Run("Invoke v3", func(t *testing.T) {
		t.Parallel()
		broadcasted, originalCoreTx := loadTx(t, sepoliaInvokeTxnPath)

		coreTx, err := rpc.AdaptBroadcastedTransactionToCore(t.Context(), broadcasted, nil, network)
		require.NoError(t, err)

		invokeTx, ok := coreTx.(*core.InvokeTransaction)
		require.True(t, ok, "expected *core.InvokeTransaction, got %T", coreTx)

		assert.Equal(t, originalCoreTx.Hash(), invokeTx.TransactionHash)
		marshalAndCompare(t, originalCoreTx, coreTx)
	})

	t.Run("Declare v3 uses provided classHash", func(t *testing.T) {
		t.Parallel()
		broadcasted, originalCoreTx := loadTx(t, sepoliaDeclareTxnPath)
		classHash := originalCoreTx.(*core.DeclareTransaction).ClassHash

		coreTx, err := rpc.AdaptBroadcastedTransactionToCore(
			t.Context(), broadcasted, classHash, network,
		)
		require.NoError(t, err)

		declareTx, ok := coreTx.(*core.DeclareTransaction)
		require.True(t, ok, "expected *core.DeclareTransaction, got %T", coreTx)
		assert.Equal(t, originalCoreTx.Hash(), declareTx.TransactionHash)
		marshalAndCompare(t, originalCoreTx, coreTx)
	})

	t.Run("DeployAccount v3", func(t *testing.T) {
		t.Parallel()
		broadcasted, originalCoreTx := loadTx(t, sepoliaDeployAccountTxnPath)

		coreTx, err := rpc.AdaptBroadcastedTransactionToCore(t.Context(), broadcasted, nil, network)
		require.NoError(t, err)

		deployAccountTx, ok := coreTx.(*core.DeployAccountTransaction)
		require.True(t, ok, "expected *core.DeployAccountTransaction, got %T", coreTx)
		assert.Equal(t, originalCoreTx.Hash(), deployAccountTx.TransactionHash)
		marshalAndCompare(t, originalCoreTx, coreTx)
	})

	t.Run("Unsupported transaction types return an error", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			txType rpc.TransactionType
		}{
			{"L1_HANDLER", rpc.TxnL1Handler},
			{"DEPLOY", rpc.TxnDeploy},
			{"Invalid", rpc.Invalid},
			{"Unknown", rpc.TransactionType(255)},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				broadcasted := &rpc.BroadcastedTransaction{
					Transaction: rpc.Transaction{Type: tc.txType},
				}
				coreTx, err := rpc.AdaptBroadcastedTransactionToCore(
					t.Context(), broadcasted, nil, network,
				)
				require.Error(t, err)
				assert.Nil(t, coreTx)
				assert.Contains(t, err.Error(), "invalid transaction type")
			})
		}
	})
}
