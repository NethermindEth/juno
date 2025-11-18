package blocktransactions

import (
	"context"
	"errors"
	"iter"
	"runtime"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/utils"
	"golang.org/x/sync/errgroup"
)

const (
	batchSize           = 10
	batchByteSize       = 128 * utils.Megabyte
	targetBatchByteSize = 96 * utils.Megabyte
	concurrency         = 4
	timeLogRate         = 5 * time.Second
	logRate             = 100
	initialTxSize       = 10 * utils.Kilobyte
	initialBlockSize    = 8 * utils.Megabyte
)

type BlockTransactionsMigration struct{}

type bufferedMigrator struct {
	written int
	data    []pooledTransaction
}

func (b *bufferedMigrator) Len() int {
	return b.written
}

func (b *bufferedMigrator) Encode(entry prefix.Entry[pooledTransaction]) error {
	b.written += entry.Value.Len()
	b.data = append(b.data, entry.Value)
	return nil
}

var (
	rawTransactions = prefix.NewPrefixedBucket(
		typed.NewBucket(
			core.TransactionsByBlockNumberAndIndexBucket.Bucket.Bucket,
			key.Bytes,
			pooledTransactionSerializer{},
		),
		prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[pooledTransaction]())),
	)
	rawReceipts = prefix.NewPrefixedBucket(
		typed.NewBucket(
			core.ReceiptsByBlockNumberAndIndexBucket.Bucket.Bucket,
			key.Bytes,
			pooledTransactionSerializer{},
		),
		prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[pooledTransaction]())),
	)
	rawBlockTransactions = core.BlockTransactionsBucket.RawValue()
)

func readBlock(
	logger utils.SimpleLogger,
	database db.KeyValueReader,
	batch db.KeyValueWriter,
	blockBufferPool *byteBufferPool,
	blockNumber uint64,
	txCount int,
) error {
	var bufferedMigrator bufferedMigrator
	defer func() {
		for _, tx := range bufferedMigrator.data {
			transactionBufferPool.put(tx.Buffer)
		}
	}()

	transactions, err := indexed.Write(
		&bufferedMigrator,
		rawTransactions.Prefix().Add(blockNumber).Scan(database),
		txCount,
	)
	if err != nil {
		return err
	}

	receipts, err := indexed.Write(
		&bufferedMigrator,
		rawReceipts.Prefix().Add(blockNumber).Scan(database),
		txCount,
	)
	if err != nil {
		return err
	}

	if len(transactions) == 0 || len(receipts) == 0 {
		has, err := core.BlockTransactionsBucket.Has(database, blockNumber)
		if err != nil {
			return err
		}
		// ALready migrated
		if has {
			logger.Infow("skipping already migrated block", "blockNumber", blockNumber)
			return nil
		}
		// Not migrated yet, no transactions found, while there are expected transactions
		if txCount > 0 {
			return errors.New("missing transactions and receipts")
		}
	}

	buf := blockBufferPool.get()
	defer blockBufferPool.put(buf)

	indexes := core.BlockTransactionsIndexes{
		Transactions: transactions,
		Receipts:     receipts,
	}
	if err := encoder.NewEncoder(buf).Encode(indexes); err != nil {
		return err
	}

	for _, tx := range bufferedMigrator.data {
		if _, err := buf.Write(tx.Buffer.Bytes()); err != nil {
			return err
		}
	}

	bytes := buf.Bytes()
	return rawBlockTransactions.Put(batch, blockNumber, &bytes)
}

func deleteOldData(database db.KeyValueRangeDeleter, startBlock, endBlock uint64) error {
	err := core.TransactionsByBlockNumberAndIndexBucket.Prefix().DeleteRange(
		database,
		startBlock,
		endBlock+1,
	)
	if err != nil {
		return err
	}

	err = core.ReceiptsByBlockNumberAndIndexBucket.Prefix().DeleteRange(
		database,
		startBlock,
		endBlock+1,
	)
	if err != nil {
		return err
	}

	return nil
}

type writeTask struct {
	batch        db.Batch
	totalTxCount int
	err          error
}

func createBatch(
	logger utils.SimpleLogger,
	database db.KeyValueStore,
	blockBufferPool *byteBufferPool,
	startBlock,
	endBlock uint64,
	batch db.Batch,
) (txCount int, err error) {
	counters, txCount, err := getTxCount(database, startBlock, endBlock)
	if err != nil {
		return 0, err
	}

	debugLog(logger, "fetching blocks", startBlock, endBlock, txCount)
	defer debugLog(logger, "fetched blocks", startBlock, endBlock, txCount)

	for blockNumber := startBlock; blockNumber <= endBlock; blockNumber++ {
		err := readBlock(
			logger,
			database,
			batch,
			blockBufferPool,
			blockNumber,
			counters[blockNumber-startBlock],
		)
		if err != nil {
			return 0, err
		}
	}

	if err := deleteOldData(batch, startBlock, endBlock); err != nil {
		return 0, err
	}

	return txCount, nil
}

func debugLog(logger utils.SimpleLogger, msg string, startBlock, endBlock uint64, txCount int) {
	if startBlock%logRate != 0 {
		return
	}

	logger.Debugw(
		msg,
		"startBlock", startBlock,
		"endBlock", endBlock,
		"txCount", txCount,
		"numGoroutines", runtime.NumGoroutine(),
	)
}

func getFirstBlockInBucket[A any](items iter.Seq2[prefix.Entry[A], error]) (uint64, bool, error) {
	for item, err := range items {
		if err != nil {
			return 0, false, err
		}

		var blockNumIndex db.BlockNumIndexKey
		if err := blockNumIndex.UnmarshalBinary(item.Key[1:]); err != nil {
			return 0, false, err
		}
		return blockNumIndex.Number, true, nil
	}
	return 0, false, nil
}

func getFirstBlock(database db.KeyValueReader) (uint64, bool, error) {
	transactions, hasTransactions, err := getFirstBlockInBucket(
		core.TransactionsByBlockNumberAndIndexBucket.Prefix().Scan(database),
	)
	if err != nil {
		return 0, false, err
	}

	receipts, hasReceipts, err := getFirstBlockInBucket(
		core.ReceiptsByBlockNumberAndIndexBucket.Prefix().Scan(database),
	)
	if err != nil {
		return 0, false, err
	}

	if !hasReceipts && !hasTransactions {
		return 0, false, nil
	}

	minBlock := min(transactions, receipts)
	minBlock -= minBlock % batchSize
	return minBlock, true, nil
}

func clearOldBuckets(database db.KeyValueStore) error {
	err := core.TransactionsByBlockNumberAndIndexBucket.Prefix().DeletePrefix(database)
	if err != nil {
		return err
	}

	err = core.ReceiptsByBlockNumberAndIndexBucket.Prefix().DeletePrefix(database)
	if err != nil {
		return err
	}

	return nil
}

func getTxCount(database db.KeyValueReader, startBlock, endBlock uint64) ([]int, int, error) {
	total := 0
	counters := make([]int, endBlock-startBlock+1)
	for blockNumber := startBlock; blockNumber <= endBlock; blockNumber++ {
		blockHeader, err := core.GetBlockHeaderByNumber(database, blockNumber)
		if err != nil {
			return nil, 0, err
		}
		total += int(blockHeader.TransactionCount)
		counters[blockNumber-startBlock] = int(blockHeader.TransactionCount)
	}
	return counters, total, nil
}

func runWriteTask(
	logger utils.SimpleLogger,
	sem chan struct{},
	counter *counter,
	task writeTask,
) error {
	if task.err != nil {
		return task.err
	}

	logger.Debugw("writing block", "txCount", task.totalTxCount)
	defer logger.Debugw("wrote block", "txCount", task.totalTxCount)
	byteSize := uint64(task.batch.Size())
	if err := task.batch.Write(); err != nil {
		return err
	}

	counter.log(byteSize, batchSize)
	sem <- struct{}{}
	return nil
}

func migrateBlockRange(
	ctx context.Context,
	database db.KeyValueStore,
	logger utils.SimpleLogger,
	firstBlock,
	chainHeight uint64,
) error {
	sem := make(chan struct{}, concurrency+1)
	for range concurrency + 1 {
		sem <- struct{}{}
	}
	blockBufferPool := newByteBufferPool(initialBlockSize)

	writer := errgroup.Group{}
	taskCh := make(chan writeTask)

	writer.Go(func() error {
		counter := newCounter(logger, timeLogRate)
		for task := range taskCh {
			if task.err != nil {
				return task.err
			}

			if err := runWriteTask(logger, sem, &counter, task); err != nil {
				return err
			}
		}
		return nil
	})

	reader := sync.WaitGroup{}
	startBlockCh := make(chan uint64)
	for range concurrency {
		reader.Go(func() {
			<-sem
			batch := database.NewBatchWithSize(batchByteSize)
			totalTxCount := 0

			for startBlock := range startBlockCh {
				endBlock := min(startBlock+batchSize-1, chainHeight)

				txCount, err := createBatch(logger, database, blockBufferPool, startBlock, endBlock, batch)
				if err != nil {
					taskCh <- writeTask{err: err}
					return
				}

				totalTxCount += txCount
				if batch.Size() >= targetBatchByteSize {
					debugLog(logger, "enqueing batch", startBlock, endBlock, txCount)
					taskCh <- writeTask{
						batch:        batch,
						totalTxCount: totalTxCount,
					}

					<-sem
					batch = database.NewBatchWithSize(batchByteSize)
					totalTxCount = 0
					debugLog(logger, "enqueued batch", startBlock, endBlock, txCount)
				} else {
					debugLog(logger, "buffered batch", startBlock, endBlock, txCount)
				}
			}

			taskCh <- writeTask{
				batch:        batch,
				totalTxCount: totalTxCount,
			}
		})
	}

outerLoop:
	for startBlock := firstBlock; startBlock <= chainHeight; startBlock += batchSize {
		select {
		case <-ctx.Done():
			break outerLoop
		case startBlockCh <- startBlock:
		}
	}

	close(startBlockCh)
	reader.Wait()

	close(taskCh)
	err := writer.Wait()

	close(sem)

	return err
}

func (BlockTransactionsMigration) Before([]byte) error {
	return nil
}

func (BlockTransactionsMigration) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	logger utils.SimpleLogger,
) ([]byte, error) {
	defer func() {
		transactionBufferPool = nil // To hint the GC to collect the pool
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, nil
		default:
		}

		firstBlock, hasFirstBlock, err := getFirstBlock(database)
		if err != nil {
			return nil, err
		}

		if !hasFirstBlock {
			logger.Infow("no starting block found, exiting")
			return nil, clearOldBuckets(database)
		}

		chainHeight, err := core.GetChainHeight(database)
		if err != nil {
			return nil, err
		}

		if err = migrateBlockRange(ctx, database, logger, firstBlock, chainHeight); err != nil {
			return []byte{}, err
		}
	}
}
