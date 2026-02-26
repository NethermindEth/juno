package proposer_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/proposer"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

const (
	maxConcurrentRead  = 8
	delayPerRead       = 100 * time.Millisecond
	waitPerTransaction = 3 * time.Second
	assertionTick      = 100 * time.Millisecond
	logLevel           = zapcore.DebugLevel
)

var allBatchSizes = []int{1, 0, 3, 2, 4, 0, 1}

func TestProposer(t *testing.T) {
	logger, err := utils.NewZapLogger(utils.NewLogLevel(logLevel), true)
	require.NoError(t, err)

	proposerAddr := starknet.Address(felt.Zero)
	bc := getBlockchain(t)
	b := getBuilder(t, logger, bc)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	toValue := func(f *felt.Felt) starknet.Value {
		return starknet.Value(*f)
	}

	// Scenario: The node build a block containing transactions from the first 3 batches then propose it
	// It continue ingesting transactions from the last 2 batches
	// Another node builds a block containing transactions from the first 2 batches and commits it
	// The first node should build a new block containing the remaining transactions and propose it
	allBatches := buildAllBatches(t, allBatchSizes)
	firstBatches := allBatches[:4]
	secondBatches := allBatches[4:]
	committedFirstBatches := allBatches[:3]
	committedSecondBatches := allBatches[3:]
	var committedValue starknet.Value

	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)

	p := proposer.New(logger, b, &proposalStore, proposerAddr, toValue)
	wg.Go(func() {
		require.NoError(t, p.Run(t.Context()))
	})

	wg.Go(runConcurrentReads(t, p))

	t.Run("Initial state", func(t *testing.T) {
		requireEventually(t, 1, func(c *assert.CollectT) {
			preconfirmed := p.Preconfirmed()
			require.NotNil(c, preconfirmed)
			assert.Empty(c, preconfirmed.Block.Transactions)
		})
	})

	t.Run("Receive transactions", func(t *testing.T) {
		for i, batch := range firstBatches {
			t.Run(fmt.Sprintf("Batch size %d", len(batch)), func(t *testing.T) {
				submit(t, p, batch)
				requireEventually(t, len(batch), func(c *assert.CollectT) {
					preconfirmed := p.Preconfirmed()
					require.NotNil(c, preconfirmed)
					assert.Equal(
						c,
						slices.Concat(firstBatches[:i+1]...),
						preconfirmed.Block.Transactions,
					)
				})
			})
		}
	})
	t.Run("Value", func(t *testing.T) {
		t.Run(fmt.Sprintf(
			"Getting value should get %d transactions", count(firstBatches)),
			func(t *testing.T) {
				assertValue(t, p, &proposalStore, firstBatches)
			},
		)

		for _, batch := range secondBatches {
			t.Run(
				fmt.Sprintf("Submitting %d more transactions", len(batch)),
				func(t *testing.T) {
					submit(t, p, batch)
				},
			)
		}

		t.Run(
			fmt.Sprintf(
				"Getting value again should still get %d transactions", count(firstBatches),
			),
			func(t *testing.T) {
				// Wait to make sure that the transactions are processed if the code is incorrect
				time.Sleep(waitPerTransaction)
				assertValue(t, p, &proposalStore, firstBatches)
			},
		)
	})

	t.Run("Another node build the first 2 batches", func(t *testing.T) {
		otherProposer := proposer.New(logger, b, &proposalStore, proposerAddr, toValue)
		wg.Go(func() {
			require.NoError(t, otherProposer.Run(t.Context()))
		})

		for i, batch := range committedFirstBatches {
			t.Run(fmt.Sprintf("Batch size %d", len(batch)), func(t *testing.T) {
				submit(t, otherProposer, batch)
				requireEventually(t, len(batch), func(c *assert.CollectT) {
					preconfirmed := otherProposer.Preconfirmed()
					require.NotNil(c, preconfirmed)
					assert.Equal(
						c,
						slices.Concat(committedFirstBatches[:i+1]...),
						preconfirmed.Block.Transactions,
					)
				})
			})
		}
		committedValue = otherProposer.Value()
	})

	t.Run("Commit", func(t *testing.T) {
		t.Run(
			fmt.Sprintf(
				"Commit the %d transactions by the other proposer", count(committedFirstBatches),
			),
			func(t *testing.T) {
				commit(t, p, &proposalStore, bc, 1, committedValue)
			},
		)

		t.Run(
			fmt.Sprintf(
				"Should process the pending %d transactions", count(committedSecondBatches),
			),
			func(t *testing.T) {
				requireEventually(t, count(committedSecondBatches), func(c *assert.CollectT) {
					preconfirmed := p.Preconfirmed()
					require.NotNil(c, preconfirmed)
					assert.Equal(
						c,
						slices.Concat(committedSecondBatches...),
						preconfirmed.Block.Transactions,
					)
				})
			},
		)

		t.Run(fmt.Sprintf("Commit the pending %d transactions", count(committedSecondBatches)), func(t *testing.T) {
			commit(t, p, &proposalStore, bc, 2, p.Value())

			requireEventually(t, 1, func(c *assert.CollectT) {
				preconfirmed := p.Preconfirmed()
				require.NotNil(c, preconfirmed)
				assert.Empty(c, preconfirmed.Block.Transactions)
			})
		})
	})
}

func runConcurrentReads(t *testing.T, p proposer.Proposer[starknet.Value, starknet.Hash]) func() {
	readCtx, readCancel := context.WithCancel(t.Context())
	pool := pool.New().WithMaxGoroutines(maxConcurrentRead).WithContext(readCtx)
	t.Cleanup(readCancel)
	return func() {
		defer func() {
			require.NoError(t, pool.Wait())
		}()
		for {
			select {
			case <-readCtx.Done():
				return
			default:
				pool.Go(func(ctx context.Context) error {
					t.Helper()
					time.Sleep(delayPerRead)
					require.NotNil(t, p.Preconfirmed())
					return nil
				})
			}
		}
	}
}

func getBlockchain(t *testing.T) *blockchain.Blockchain {
	t.Helper()
	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network)
	return bc
}

func getBuilder(t *testing.T, log utils.Logger, bc *blockchain.Blockchain) *builder.Builder {
	t.Helper()

	genesisConfig, err := genesis.Read("../../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../../genesis/classes/strk.json", "../../genesis/classes/account.json",
		"../../genesis/classes/universaldeployer.json", "../../genesis/classes/udacnt.json",
	}

	feeTokens := utils.DefaultFeeTokenAddresses
	chainInfo := vm.ChainInfo{
		ChainID:           bc.Network().L2ChainID,
		FeeTokenAddresses: feeTokens,
	}
	diff, classes, err := genesis.GenesisStateDiff(
		t.Context(),
		genesisConfig,
		vm.New(&chainInfo, false, log),
		bc.Network(),
		vm.DefaultMaxSteps,
		vm.DefaultMaxGas,
		compiler.NewUnsafe(),
	)
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	executor := builder.NewExecutor(bc, vm.New(&chainInfo, false, log), log, false, true)
	testBuilder := builder.New(bc, executor)
	return &testBuilder
}

func count[T any](batchSizes [][]T) int {
	total := 0
	for _, size := range batchSizes {
		total += len(size)
	}
	return total
}

func buildAllBatches(t *testing.T, batchSizes []int) [][]core.Transaction {
	t.Helper()

	batches := make([][]core.Transaction, len(batchSizes))
	nonce := 0

	for i, size := range batchSizes {
		batches[i] = make([]core.Transaction, size)
		for j := range batches[i] {
			batches[i][j] = buildRandomTransaction(t, uint64(nonce))
			nonce++
		}
	}

	return batches
}

func buildRandomTransaction(t *testing.T, nonce uint64) core.Transaction {
	t.Helper()
	hash := felt.FromUint64[felt.Felt](nonce)

	return &core.InvokeTransaction{
		TransactionHash: &hash,
		SenderAddress:   felt.NewUnsafeFromString[felt.Felt]("0x101"),
		Version:         new(core.TransactionVersion).SetUint64(3),
		Nonce:           new(felt.Felt).SetUint64(nonce),
		TransactionSignature: []*felt.Felt{
			felt.NewUnsafeFromString[felt.Felt]("0xa678c78ff34d4a0ccd5063318265d60e233445782892b40e019bf4556e57c0"),
			felt.NewUnsafeFromString[felt.Felt]("0x234470d2c4f6dc6f8e38adf1992cda3969119f62f25941b8bfb4ccd50b5c823"),
		},
		CallData: []*felt.Felt{
			felt.NewUnsafeFromString[felt.Felt]("0x1"),
			felt.NewUnsafeFromString[felt.Felt]("0x4718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
			felt.NewUnsafeFromString[felt.Felt]("0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
			felt.NewUnsafeFromString[felt.Felt]("0x3"),
			felt.NewUnsafeFromString[felt.Felt]("0x105"),
			felt.NewUnsafeFromString[felt.Felt]("0x1234"),
			felt.NewUnsafeFromString[felt.Felt]("0x0"),
		},
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas: {
				MaxAmount:       4,
				MaxPricePerUnit: new(felt.Felt).SetUint64(1),
			},
			core.ResourceL2Gas: {
				MaxAmount:       520000,
				MaxPricePerUnit: new(felt.Felt).SetUint64(1),
			},
			core.ResourceL1DataGas: {
				MaxAmount:       296,
				MaxPricePerUnit: new(felt.Felt).SetUint64(1),
			},
		},
		Tip:                   0x0,
		PaymasterData:         []*felt.Felt{},
		AccountDeploymentData: []*felt.Felt{},
		NonceDAMode:           core.DAModeL1,
		FeeDAMode:             core.DAModeL1,
	}
}

func submit(t *testing.T, proposer proposer.Proposer[starknet.Value, starknet.Hash], batch []core.Transaction) {
	t.Helper()
	transactions := make([]mempool.BroadcastedTransaction, len(batch))
	for i, transaction := range batch {
		transactions[i] = mempool.BroadcastedTransaction{
			Transaction: transaction,
		}
	}

	proposer.Submit(t.Context(), transactions)
}

func assertValue(
	t *testing.T,
	proposer proposer.Proposer[starknet.Value, starknet.Hash],
	proposalStore *proposal.ProposalStore[starknet.Hash],
	expected [][]core.Transaction,
) {
	t.Helper()
	value := proposer.Value()
	assert.NotNil(t, value)
	assert.True(t, proposer.Valid(value))

	buildResult := proposalStore.Get(value.Hash())
	assert.NotNil(t, buildResult)
	assert.Equal(t, slices.Concat(expected...), buildResult.Preconfirmed.Block.Transactions)
}

func commit(
	t *testing.T,
	proposer proposer.Proposer[starknet.Value, starknet.Hash],
	proposalStore *proposal.ProposalStore[starknet.Hash],
	bc *blockchain.Blockchain,
	height types.Height,
	committedValue starknet.Value,
) {
	t.Helper()
	result := proposalStore.Get(committedValue.Hash())
	require.NotNil(t, result)

	require.NoError(t, bc.Store(
		result.Preconfirmed.Block,
		result.SimulateResult.BlockCommitments,
		result.Preconfirmed.StateUpdate,
		result.Preconfirmed.NewClasses,
	))

	proposer.OnCommit(t.Context(), height, committedValue)
}

func requireEventually(t *testing.T, transactionCount int, condition func(c *assert.CollectT)) {
	require.EventuallyWithT(
		t,
		condition,
		time.Duration(transactionCount+1)*waitPerTransaction,
		assertionTick,
	)
}
