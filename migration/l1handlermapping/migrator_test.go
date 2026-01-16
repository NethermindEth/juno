package l1handlermapping_test

import (
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/l1handlermapping"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestRecalculateL1HandlerMsgHashesToTxnHashes(t *testing.T) {
	t.Run("empty DB", func(t *testing.T) {
		testdb := memory.New()
		m := l1handlermapping.Migrator{}
		require.NoError(t, m.Before(nil))
		intermediateState, err := m.Migrate(t.Context(), testdb, &utils.Sepolia, utils.NewNopZapLogger())
		require.NoError(t, err)
		require.Nil(t, intermediateState)
	})

	t.Run("calculate L1Handler transactions per block with mixed tx types", func(t *testing.T) {
		testdb := memory.New()

		const numBlocks uint64 = 10
		const minTxsPerBlock = 5
		const maxTxsPerBlock = 15

		// Track all L1Handler transactions for verification
		type l1HandlerInfo struct {
			msgHash []byte
			txHash  *felt.Felt
		}
		var allL1Handlers []l1HandlerInfo

		// Use weights with higher L1Handler probability to ensure we get some
		weights := txWeights{
			Invoke:        0.30,
			DeployAccount: 0.10,
			Declare:       0.10,
			L1Handler:     0.50,
		}

		batch := testdb.NewBatch()

		for blockNum := range numBlocks {
			// Random number of transactions per block
			numTxs := minTxsPerBlock + rand.Intn(maxTxsPerBlock-minTxsPerBlock+1)

			// Generate transactions with weighted sampling
			txs, receipts := generateWeightedTestTxs(t, &utils.Sepolia, numTxs, weights)

			// Write transactions and track L1Handlers
			for txIndex, tx := range txs {
				require.NoError(t, core.WriteTxAndReceipt(
					batch,
					blockNum,
					uint64(txIndex),
					tx,
					receipts[txIndex],
				))

				// Track L1Handler transactions for verification
				if l1Handler, ok := tx.(*core.L1HandlerTransaction); ok {
					allL1Handlers = append(allL1Handlers, l1HandlerInfo{
						msgHash: l1Handler.MessageHash(),
						txHash:  l1Handler.Hash(),
					})
				}
			}
		}

		// Ensure we have at least some L1Handlers to verify
		require.NotEmpty(t, allL1Handlers, "should have generated some L1Handler transactions")

		require.NoError(t, core.WriteChainHeight(batch, numBlocks-1))
		require.NoError(t, batch.Write())

		// Verify bucket is empty (WriteTxAndReceipt didn't write them)
		iter, err := testdb.NewIterator(db.L1HandlerTxnHashByMsgHash.Key(), true)
		require.NoError(t, err)
		defer iter.Close()
		require.False(
			t,
			iter.First(),
			"L1HandlerTxnHashByMsgHash bucket should be empty before migration",
		)

		// Run migration
		m := l1handlermapping.Migrator{}
		require.NoError(t, m.Before(nil))
		intermediateState, err := m.Migrate(t.Context(), testdb, &utils.Sepolia, utils.NewNopZapLogger())
		require.NoError(t, err)
		require.Nil(t, intermediateState)

		// Verify ALL L1Handler mappings are now present and correct
		for _, info := range allL1Handlers {
			mappedTxHash, err := core.GetL1HandlerTxnHashByMsgHash(testdb, info.msgHash)
			require.NoError(t, err)
			require.True(t, mappedTxHash.Equal(info.txHash))
		}
	})
}

// txWeights defines weights for each transaction type
type txWeights struct {
	Declare       float64
	DeployAccount float64
	Invoke        float64
	L1Handler     float64
}

// generateWeightedTestTxs generates n random transactions with receipts based on weights
func generateWeightedTestTxs(
	t *testing.T,
	network *utils.Network,
	n int,
	weights txWeights,
) ([]core.Transaction, []*core.TransactionReceipt) {
	t.Helper()

	builder := &testutils.SyncTransactionBuilder[core.Transaction, struct{}]{
		ToCore: func(tx core.Transaction, _ core.ClassDefinition, _ *felt.Felt) core.Transaction {
			return tx
		},
	}

	// Build cumulative distribution
	cumulative := []float64{
		weights.Declare,
		weights.Declare + weights.DeployAccount,
		weights.Declare + weights.DeployAccount + weights.Invoke,
		weights.Declare + weights.DeployAccount + weights.Invoke + weights.L1Handler,
	}
	total := cumulative[3]

	txs := make([]core.Transaction, n)
	receipts := make([]*core.TransactionReceipt, n)

	for i := range n {
		r := rand.Float64() * total

		var tx core.Transaction
		switch {
		case r < cumulative[0]:
			tx, _ = builder.GetTestDeclareV3Transaction(t, network)
		case r < cumulative[1]:
			tx, _ = builder.GetTestDeployAccountTransactionV3(t, network)
		case r < cumulative[2]:
			tx, _ = builder.GetTestInvokeTransactionV3(t, network)
		default:
			// L1Handler: retry until non-empty CallData
			for {
				tx, _ = builder.GetTestL1HandlerTransaction(t, network)
				if len(tx.(*core.L1HandlerTransaction).CallData) > 0 {
					break
				}
			}
		}

		receipts[i] = &core.TransactionReceipt{
			TransactionHash: tx.Hash(),
			Fee:             felt.NewRandom[felt.Felt](),
			FeeUnit:         core.WEI,
		}
		txs[i] = tx
	}

	return txs, receipts
}
