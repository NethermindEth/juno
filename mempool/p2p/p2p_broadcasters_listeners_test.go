package p2p_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/mempool2p2p/testutils"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mempool/p2p"
	pubsubtestutils "github.com/NethermindEth/juno/p2p/pubsub/testutils"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

const (
	logLevel  = zapcore.InfoLevel
	nodeCount = 20
	txCount   = 50
	maxWait   = 30 * time.Second
)

type origin struct {
	Source int
	Index  int
}

// mockMempool implements mempool.Pool for testing
type mockMempool chan<- *mempool.BroadcastedTransaction

func (m mockMempool) Push(ctx context.Context, tx *mempool.BroadcastedTransaction) error {
	m <- tx
	return nil
}

func TestMempoolBroadcastersAndListeners(t *testing.T) {
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	transactions := make([][]mempool.BroadcastedTransaction, nodeCount)
	iter.ForEach(transactions, func(txs *[]mempool.BroadcastedTransaction) {
		*txs = getRandomTransactions(t)
	})

	nodes := pubsubtestutils.BuildNetworks(t, pubsubtestutils.LineNetworkConfig(nodeCount))

	txSet := make(map[string]origin)
	for node := range nodeCount {
		for index, tx := range transactions[node] {
			txSet[tx.Transaction.Hash().String()] = origin{Source: node, Index: index}
		}
	}

	transactionWait := conc.NewWaitGroup()
	peerWait := conc.NewWaitGroup()
	for index, node := range nodes {
		logger := logger.Named(fmt.Sprint(index))

		received := make(chan *mempool.BroadcastedTransaction, txCount)
		pool := mockMempool(received)

		p2p := p2p.New(&utils.Mainnet, node.Host, logger, &pool, &config.DefaultBufferSizes, node.GetBootstrapPeers)

		peerWait.Go(func() {
			require.NoError(t, p2p.Run(t.Context()))
		})

		transactionWait.Go(func() {
			for i, transaction := range transactions[index] {
				logger.Debug("sending", utils.SugaredFields("count", i)...)
				require.NoError(t, p2p.Push(t.Context(), &transaction))
			}
		})

		transactionWait.Go(func() {
			pending := maps.Clone(txSet)

			// Ignore the transactions we are broadcasting
			for _, transaction := range transactions[index] {
				delete(pending, transaction.Transaction.Hash().String())
			}

			for {
				select {
				case transaction := <-received:
					delete(pending, transaction.Transaction.Hash().String())
					logger.Debug("pending", utils.SugaredFields("count", len(pending))...)
					if len(pending) == 0 {
						logger.Info("all transactions received")
						return
					}
				case <-time.After(maxWait):
					logger.Info("missing transactions", utils.SugaredFields("pending", slices.Collect(maps.Values(pending)))...)
					require.FailNow(t, "timed out waiting for transactions")
				}
			}
		})
	}

	t.Cleanup(peerWait.Wait)
	transactionWait.Wait()
}

func getRandomTransactions(t *testing.T) []mempool.BroadcastedTransaction {
	transactions := make([]mempool.BroadcastedTransaction, txCount)
	for i := range txCount {
		transactions[i], _ = testutils.TransactionBuilder.GetTestInvokeTransaction(t, &utils.Mainnet)
	}
	return transactions
}
