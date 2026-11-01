package blocktransactions_test

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/migration/blocktransactions"
	"github.com/NethermindEth/juno/utils/log"
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
	t.Run("Empty database", func(t *testing.T) {
		database := openDatabase(t, t.TempDir())

		state, err := blocktransactions.Migrator{}.Migrate(
			t.Context(),
			database,
			&networks.Sepolia,
			log.NewNopZapLogger(),
		)
		require.NoError(t, err)
		require.Nil(t, state)
	})

	t.Run("Existing database", func(t *testing.T) {
		counts, allTransactions, allReceipts := generateFixtures(t)

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
	})
}

func generateFixtures(t *testing.T) ([]int, [][]core.Transaction, [][]*core.TransactionReceipt) {
	t.Helper()
	counts := make([]int, chainHeight+1)
	allTransactions := make([][]core.Transaction, chainHeight+1)
	allReceipts := make([][]*core.TransactionReceipt, chainHeight+1)

	conciter.ForEachIdx(counts, func(blockNumber int, count *int) {
		*count = rand.IntN(maxTransactions-minTransactions) + minTransactions
		allTransactions[blockNumber] = testutils.GetCoreTransactions(t, *count)
		allReceipts[blockNumber] = testutils.GetCoreReceipts(t, *count)
	})

	return counts, allTransactions, allReceipts
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
		database := openDatabase(t, dir)

		require.NoError(
			t,
			core.ChainHeightBucket.Put(database, struct{}{}, new(chainHeight)),
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
			require.NoError(
				t,
				writePerTxTransactionsAndReceipts(
					batch,
					uint64(blockNumber),
					allTransactions[blockNumber],
					allReceipts[blockNumber],
				),
			)
		})
	})

	t.Run("Read old data", func(t *testing.T) {
		snapshot := openSnapshot(t, dir)

		t.Run("PerTx layout should contain data", func(t *testing.T) {
			assertPerTxData(
				t,
				snapshot,
				allTransactions,
				allReceipts,
			)
		})

		t.Run("Combined layout should be empty", func(t *testing.T) {
			assertEmptyBucket(t, core.BlockTransactionsBucket.Prefix().Scan(snapshot))
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
				database := openDatabase(t, dir)

				logger, err := log.NewZapLogger(log.NewLevel(zapcore.InfoLevel), log.WithColour(true))
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
					&networks.Sepolia,
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
					snapshot := openSnapshot(t, dir)

					t.Run("Combined layout should contain data", func(t *testing.T) {
						assertCombinedData(
							t,
							snapshot,
							allTransactions,
							allReceipts,
						)
					})

					t.Run("PerTx layout should be empty", func(t *testing.T) {
						assertEmptyBucket(
							t,
							core.DeprecatedTransactionsByBlockNumberAndIndexBucket.Prefix().Scan(snapshot),
						)
						assertEmptyBucket(
							t,
							core.DeprecatedReceiptsByBlockNumberAndIndexBucket.Prefix().Scan(snapshot),
						)
					})
				})
			}
		}
	})
}

//nolint:staticcheck // Intentionally testing deprecated per-tx layout buckets.
func writePerTxTransactionsAndReceipts(
	w db.KeyValueWriter,
	blockNumber uint64,
	transactions []core.Transaction,
	receipts []*core.TransactionReceipt,
) error {
	for index, tx := range transactions {
		txHash := (*felt.TransactionHash)(tx.Hash())
		key := db.BlockNumIndexKey{
			Number: blockNumber,
			Index:  uint64(index),
		}

		if err := core.TransactionBlockNumbersAndIndicesByHashBucket.Put(
			w,
			txHash,
			&key,
		); err != nil {
			return err
		}

		if err := core.DeprecatedTransactionsByBlockNumberAndIndexBucket.Put(
			w,
			key,
			&tx,
		); err != nil {
			return err
		}

		if err := core.DeprecatedReceiptsByBlockNumberAndIndexBucket.Put(
			w,
			key,
			receipts[index],
		); err != nil {
			return err
		}
	}
	return nil
}

//nolint:staticcheck // Intentionally reading from deprecated per-tx layout bucket.
func perTxTransactionsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]core.Transaction, error) {
	var transactions []core.Transaction
	iterator := core.DeprecatedTransactionsByBlockNumberAndIndexBucket.
		Prefix().
		Add(blockNum).
		Scan(r)
	for entry, err := range iterator {
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, entry.Value)
	}
	return transactions, nil
}

//nolint:staticcheck // Intentionally reading from deprecated per-tx layout bucket.
func perTxTransactionByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber, index uint64,
) (core.Transaction, error) {
	key := db.BlockNumIndexKey{
		Number: blockNumber,
		Index:  index,
	}
	return core.DeprecatedTransactionsByBlockNumberAndIndexBucket.Get(r, key)
}

//nolint:staticcheck // Intentionally reading from deprecated per-tx layout bucket.
func perTxReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]*core.TransactionReceipt, error) {
	var receipts []*core.TransactionReceipt
	iterator := core.DeprecatedReceiptsByBlockNumberAndIndexBucket.
		Prefix().
		Add(blockNum).
		Scan(r)
	for entry, err := range iterator {
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, &entry.Value)
	}
	return receipts, nil
}

//nolint:staticcheck // Intentionally reading from deprecated per-tx layout bucket.
func perTxReceiptByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber, index uint64,
) (*core.TransactionReceipt, error) {
	key := db.BlockNumIndexKey{
		Number: blockNumber,
		Index:  index,
	}
	receipt, err := core.DeprecatedReceiptsByBlockNumberAndIndexBucket.Get(r, key)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func combinedTransactionsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]core.Transaction, error) {
	return core.BlockTransactionsAllTransactionsPartialBucket.Get(r, blockNum, struct{}{})
}

func combinedTransactionByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber, index uint64,
) (core.Transaction, error) {
	return core.BlockTransactionsTransactionPartialBucket.Get(r, blockNumber, int(index))
}

func combinedReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]*core.TransactionReceipt, error) {
	return core.BlockTransactionsAllReceiptsPartialBucket.Get(r, blockNum, struct{}{})
}

func combinedReceiptByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber, index uint64,
) (*core.TransactionReceipt, error) {
	return core.BlockTransactionsReceiptPartialBucket.Get(r, blockNumber, int(index))
}

func assertPerTxData(
	t *testing.T,
	snapshot db.KeyValueReader,
	allTransactions [][]core.Transaction,
	allReceipts [][]*core.TransactionReceipt,
) {
	t.Helper()

	t.Run("Transactions", func(t *testing.T) {
		assertCurrentLayout(
			t,
			snapshot,
			allTransactions,
			perTxTransactionsByBlockNumber,
			perTxTransactionByBlockAndIndex,
		)
	})

	t.Run("Receipts", func(t *testing.T) {
		assertCurrentLayout(
			t,
			snapshot,
			allReceipts,
			perTxReceiptsByBlockNumber,
			perTxReceiptByBlockAndIndex,
		)
	})
}

func assertCombinedData(
	t *testing.T,
	snapshot db.KeyValueReader,
	allTransactions [][]core.Transaction,
	allReceipts [][]*core.TransactionReceipt,
) {
	t.Helper()

	t.Run("Transactions", func(t *testing.T) {
		assertCurrentLayout(
			t,
			snapshot,
			allTransactions,
			combinedTransactionsByBlockNumber,
			combinedTransactionByBlockAndIndex,
		)
	})

	t.Run("Receipts", func(t *testing.T) {
		assertCurrentLayout(
			t,
			snapshot,
			allReceipts,
			combinedReceiptsByBlockNumber,
			combinedReceiptByBlockAndIndex,
		)
	})
}

func assertCurrentLayout[T any](
	t *testing.T,
	snapshot db.KeyValueReader,
	expected [][]T,
	getByBlock func(database db.KeyValueReader, blockNumber uint64) ([]T, error),
	getByBlockAndIndex func(database db.KeyValueReader, blockNumber uint64, index uint64) (T, error),
) {
	t.Run("All", func(t *testing.T) {
		conciter.ForEachIdx(expected, func(blockNumber int, expected *[]T) {
			actual, err := getByBlock(snapshot, uint64(blockNumber))
			require.NoError(t, err)
			require.Len(t, actual, len(*expected))
			if len(*expected) > 0 {
				require.Equal(t, *expected, actual)
			}
		})
	})
	t.Run("Per index", func(t *testing.T) {
		conciter.ForEachIdx(expected, func(blockNumber int, expected *[]T) {
			conciter.ForEachIdx(*expected, func(index int, expected *T) {
				actual, err := getByBlockAndIndex(
					snapshot,
					uint64(blockNumber),
					uint64(index),
				)
				require.NoError(t, err)
				require.Equal(t, *expected, actual)
			})
		})
	})
}

func openDatabase(t *testing.T, dir string) db.KeyValueStore {
	t.Helper()
	database, err := pebblev2.New(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, database.Close())
	})
	return database
}

func openSnapshot(t *testing.T, dir string) db.Snapshot {
	t.Helper()
	database := openDatabase(t, dir)
	snapshot := database.NewSnapshot()
	t.Cleanup(func() {
		require.NoError(t, snapshot.Close())
	})
	return snapshot
}

func assertEmptyBucket[T any](
	t *testing.T,
	bucket iter.Seq2[prefix.Entry[T], error],
) {
	t.Helper()
	next, stop := iter.Pull2(bucket)
	defer stop()
	_, _, ok := next()
	require.False(t, ok)
}
