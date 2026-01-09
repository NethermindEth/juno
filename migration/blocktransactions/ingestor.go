package blocktransactions

import (
	"errors"
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/utils"
	"github.com/fxamacker/cbor/v2"
)

type ingestor struct {
	logger        utils.SimpleLogger
	database      db.KeyValueReader
	chainHeight   uint64
	batchProvider *batchProvider
	tasks         []task
}

func newIngestor(
	logger utils.SimpleLogger,
	database db.KeyValueReader,
	chainHeight uint64,
	batchProvider *batchProvider,
) *ingestor {
	tasks := make([]task, ingestorCount)
	for i := range tasks {
		tasks[i] = batchProvider.get()
	}
	return &ingestor{
		logger:        logger,
		database:      database,
		chainHeight:   chainHeight,
		batchProvider: batchProvider,
		tasks:         tasks,
	}
}

var _ state[uint64, task] = (*ingestor)(nil)

func (c *ingestor) run(index int, startBlock uint64, outputs chan<- task) error {
	if startBlock%logRate == 0 {
		c.logger.Infow("Migrating block", "startBlock", startBlock)
	}

	endBlock := min(startBlock+batchSize-1, c.chainHeight)
	t := &c.tasks[index]

	txCount, err := c.ingestBlockRange(t.batch, startBlock, endBlock)
	if err != nil {
		return err
	}

	t.totalTxCount += txCount
	t.totalBlockCount += int(endBlock - startBlock + 1)

	if t.batch.Size() >= targetBatchByteSize {
		outputs <- *t
		c.tasks[index] = c.batchProvider.get()
		return nil
	}

	return nil
}

func (c *ingestor) done(index int, outputs chan<- task) error {
	outputs <- c.tasks[index]
	return nil
}

func (c *ingestor) ingestBlockRange(
	batch db.Batch,
	startBlock,
	endBlock uint64,
) (int, error) {
	totalTxCount := 0

	for blockNumber := startBlock; blockNumber <= endBlock; blockNumber++ {
		txCount, err := c.ingestBlock(batch, blockNumber)
		if err != nil {
			return 0, err
		}
		totalTxCount += txCount
	}

	if err := deleteOldBlockRangeData(batch, startBlock, endBlock); err != nil {
		return 0, err
	}

	return totalTxCount, nil
}

func (c *ingestor) ingestBlock(batch db.KeyValueWriter, blockNumber uint64) (int, error) {
	blockHeader, err := core.GetBlockHeaderByNumber(c.database, blockNumber)
	if err != nil {
		return 0, err
	}
	txCount := int(blockHeader.TransactionCount)

	blockTransactions, err := core.NewBlockTransactionsFromIterators(
		extractValues(rawTransactions.Prefix().Add(blockNumber).Scan(c.database)),
		extractValues(rawReceipts.Prefix().Add(blockNumber).Scan(c.database)),
	)
	if err != nil {
		return 0, err
	}

	err = c.validateCount(
		blockNumber,
		txCount,
		len(blockTransactions.Indexes.Transactions),
		len(blockTransactions.Indexes.Receipts),
	)
	if err != nil {
		return 0, err
	}

	return txCount, core.BlockTransactionsBucket.Put(batch, blockNumber, &blockTransactions)
}

func (c *ingestor) validateCount(
	blockNumber uint64,
	txCount int,
	fetchedTxCount,
	fetchedReceiptCount int,
) error {
	if fetchedTxCount == 0 || fetchedReceiptCount == 0 {
		has, err := core.BlockTransactionsBucket.Has(c.database, blockNumber)
		if err != nil {
			return err
		}
		// Already migrated
		if has {
			c.logger.Infow("skipping already migrated block", "blockNumber", blockNumber)
			return nil
		}
		// Not migrated yet, no transactions found, while there are expected transactions
		if txCount > 0 {
			return errors.New("missing transactions and receipts")
		}
	}

	if fetchedTxCount != txCount {
		return fmt.Errorf("invalid transactions: expected %d, got %d", txCount, fetchedTxCount)
	}

	if fetchedReceiptCount != txCount {
		return fmt.Errorf("invalid receipts: expected %d, got %d", txCount, fetchedReceiptCount)
	}

	return nil
}

func extractValues(seq iter.Seq2[prefix.Entry[[]byte], error]) iter.Seq2[cbor.RawMessage, error] {
	return func(yield func(cbor.RawMessage, error) bool) {
		for item, err := range seq {
			if !yield(cbor.RawMessage(item.Value), err) {
				return
			}
		}
	}
}

func deleteOldBlockRangeData(batch db.KeyValueRangeDeleter, startBlock, endBlock uint64) error {
	err := core.TransactionsByBlockNumberAndIndexBucket.Prefix().DeleteRange(
		batch,
		startBlock,
		endBlock+1,
	)
	if err != nil {
		return err
	}

	return core.ReceiptsByBlockNumberAndIndexBucket.Prefix().DeleteRange(
		batch,
		startBlock,
		endBlock+1,
	)
}
