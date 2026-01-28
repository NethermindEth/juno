package blocktransactions_test

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/migration/blocktransactions"
	"github.com/NethermindEth/juno/utils"
	conciter "github.com/sourcegraph/conc/iter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

const (
	delay           = 200 * time.Millisecond
	chainHeight     = uint64(5000)
	minTransactions = 0
	maxTransactions = 50
)

func TestBlockTransactionsMigration(t *testing.T) {
	counts := make([]int, chainHeight+1)
	allTransactions := make([][]core.Transaction, chainHeight+1)
	allReceipts := make([][]*core.TransactionReceipt, chainHeight+1)

	t.Run("Empty database", func(t *testing.T) {
		database, err := pebblev2.New(t.TempDir())
		require.NoError(t, err)
		defer database.Close()

		state, err := blocktransactions.Migrator{}.Migrate(
			t.Context(),
			database,
			&utils.Sepolia,
			utils.NewNopZapLogger(),
		)
		require.NoError(t, err)
		require.Nil(t, state)
	})

	t.Run("Generate data", func(t *testing.T) {
		conciter.ForEachIdx(counts, func(blockNumber int, count *int) {
			*count = rand.IntN(maxTransactions-minTransactions) + minTransactions
			allTransactions[blockNumber] = testutils.GetCoreTransactions(t, *count)
			allReceipts[blockNumber] = testutils.GetCoreReceipts(t, *count)
		})
	})

	t.Run("No cancellation", func(t *testing.T) {
		runTestBlockTransactionsMigration(
			t,
			counts,
			allTransactions,
			allReceipts,
			[]bool{false, false},
		)
	})

	t.Run("With cancellation", func(t *testing.T) {
		runTestBlockTransactionsMigration(
			t,
			counts,
			allTransactions,
			allReceipts,
			[]bool{true, true, false, false},
		)
	})
}

func runTestBlockTransactionsMigration(
	t *testing.T,
	counts []int,
	allTransactions [][]core.Transaction,
	allReceipts [][]*core.TransactionReceipt,
	shouldCancelRuns []bool,
) {
	dir := t.TempDir()

	t.Run("Init", func(t *testing.T) {
		database, err := pebblev2.New(dir)
		require.NoError(t, err)
		defer database.Close()

		require.NoError(
			t,
			core.ChainHeightBucket.Put(database, struct{}{}, utils.HeapPtr(chainHeight)),
		)

		conciter.ForEachIdx(counts, func(blockNumber int, count *int) {
			batch := database.NewBatch()
			defer func() {
				require.NoError(t, batch.Write())
			}()

			blockHeader := core.Header{
				Number:           uint64(blockNumber),
				TransactionCount: uint64(*count),
			}
			require.NoError(
				t,
				core.BlockHeadersByNumberBucket.Put(batch, uint64(blockNumber), &blockHeader),
			)

			for i := range *count {
				key := db.BlockNumIndexKey{
					Number: uint64(blockNumber),
					Index:  uint64(i),
				}
				require.NoError(
					t,
					core.TransactionsByBlockNumberAndIndexBucket.Put(
						batch,
						key,
						&allTransactions[blockNumber][i],
					),
				)
				require.NoError(
					t,
					core.ReceiptsByBlockNumberAndIndexBucket.Put(
						batch,
						key,
						allReceipts[blockNumber][i],
					),
				)
			}
		})
	})

	t.Run("Read old data", func(t *testing.T) {
		database, err := pebblev2.New(dir)
		require.NoError(t, err)
		defer database.Close()

		t.Run("Transactions", func(t *testing.T) {
			assertOldEntries(
				t,
				db.TransactionsByBlockNumberAndIndex,
				counts,
				func(blockNumber int) iter.Seq2[prefix.Entry[core.Transaction], error] {
					return core.TransactionsByBlockNumberAndIndexBucket.
						Prefix().
						Add(uint64(blockNumber)).
						Scan(database)
				},
				func(t *testing.T, blockNumber, index int, value *core.Transaction) {
					require.Equal(t, allTransactions[blockNumber][index], *value)
				},
			)
		})

		t.Run("Receipts", func(t *testing.T) {
			assertOldEntries(
				t,
				db.ReceiptsByBlockNumberAndIndex,
				counts,
				func(blockNumber int) iter.Seq2[prefix.Entry[core.TransactionReceipt], error] {
					return core.ReceiptsByBlockNumberAndIndexBucket.
						Prefix().
						Add(uint64(blockNumber)).
						Scan(database)
				},
				func(t *testing.T, blockNumber, index int, value *core.TransactionReceipt) {
					require.Equal(t, allReceipts[blockNumber][index], value)
				},
			)
		})
	})

	t.Run("Migrate", func(t *testing.T) {
		finished := false
		for i, shouldCancel := range shouldCancelRuns {
			name := fmt.Sprintf("Run %d", i)
			if shouldCancel {
				name += " (cancelled)"
			}
			t.Run(name, func(t *testing.T) {
				database, err := pebblev2.New(dir)
				require.NoError(t, err)
				defer database.Close()

				logger, err := utils.NewZapLogger(utils.NewLogLevel(zapcore.InfoLevel), true)
				require.NoError(t, err)

				ctx := t.Context()

				if shouldCancel {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)

					go func() {
						time.Sleep(delay)
						cancel()
					}()
				}

				state, err := blocktransactions.Migrator{}.Migrate(
					ctx,
					database,
					&utils.Sepolia,
					logger,
				)

				require.NoError(t, err)
				if shouldCancel {
					require.Equal(t, []byte{}, state)
				} else {
					require.Nil(t, state)
					finished = true
				}
			})

			if finished {
				t.Run(fmt.Sprintf("Verify after run %d", i), func(t *testing.T) {
					database, err := pebblev2.New(dir)
					require.NoError(t, err)
					defer database.Close()

					conciter.ForEachIdx(counts, func(blockNumber int, count *int) {
						blockTransactions, err := core.BlockTransactionsBucket.Get(database, uint64(blockNumber))
						require.NoError(t, err)

						transactions, err := blockTransactions.Transactions().All()
						require.NoError(t, err)
						require.Equal(t, allTransactions[blockNumber], transactions)

						receipts, err := blockTransactions.Receipts().All()
						require.NoError(t, err)
						require.Equal(t, allReceipts[blockNumber], receipts)
					})
				})
			}
		}
	})
}

func assertOldEntries[T any](
	t *testing.T,
	bucket db.Bucket,
	counts []int,
	scan func(blockNumber int) iter.Seq2[prefix.Entry[T], error],
	assertValue func(t *testing.T, blockNumber, index int, value *T),
) {
	t.Helper()

	conciter.ForEachIdx(counts, func(blockNumber int, count *int) {
		index := 0
		for entry, err := range scan(blockNumber) {
			require.NoError(t, err)
			require.Less(t, index, *count)
			key := db.BlockNumIndexKey{
				Number: uint64(blockNumber),
				Index:  uint64(index),
			}
			require.Equal(t, bucket.Key(key.Marshal()), entry.Key)
			assertValue(t, blockNumber, index, &entry.Value)
			index++
		}
		require.Equal(t, *count, index)
	})
}
