package blocktransactions

import (
	"context"
	"errors"
	"iter"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/prefix"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/utils"
	"github.com/fxamacker/cbor/v2"
	"github.com/sourcegraph/conc/stream"
)

const (
	batchSize        = 100
	batchConcurrency = 4
	timeLogRate      = 5 * time.Second
)

type BlockTransactionsMigration struct{}

type blockTransactions struct {
	Transactions []cbor.RawMessage `cbor:"1,keyasint,omitempty"`
	Receipts     []cbor.RawMessage `cbor:"2,keyasint,omitempty"`
}

var (
	rawTransactions = prefix.NewPrefixedBucket(
		core.TransactionsByBlockNumberAndIndexBucket.RawKey().RawValue(),
		prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[[]byte]())),
	)
	rawReceipts = prefix.NewPrefixedBucket(
		core.ReceiptsByBlockNumberAndIndexBucket.RawKey().RawValue(),
		prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[[]byte]())),
	)
	rawBlockTransactions = core.BlockTransactionsBucket.RawValue()
)

func collect(iter iter.Seq2[prefix.Entry[[]byte], error]) ([]cbor.RawMessage, error) {
	items := []cbor.RawMessage{}
	for item, err := range iter {
		if err != nil {
			return nil, err
		}
		items = append(items, item.Value)
	}
	return items, nil
}

func readBlock(database db.KeyValueStore, blockNumber uint64) (blockTransactions, bool, error) {
	transactions, err := collect(rawTransactions.Prefix().Add(blockNumber).Scan(database))
	if err != nil {
		return blockTransactions{}, false, err
	}

	receipts, err := collect(rawReceipts.Prefix().Add(blockNumber).Scan(database))
	if err != nil {
		return blockTransactions{}, false, err
	}

	if len(transactions) == 0 || len(receipts) == 0 {
		has, err := core.BlockTransactionsBucket.Has(database, blockNumber)
		if err != nil {
			return blockTransactions{}, false, err
		}
		if has {
			return blockTransactions{}, false, nil
		}
	}

	return blockTransactions{
		Transactions: transactions,
		Receipts:     receipts,
	}, true, nil
}

func writeBlock(
	database db.KeyValueWriter,
	blockNumber uint64,
	blockTransactions blockTransactions,
) error {
	bytes, err := encoder.Marshal(blockTransactions)
	if err != nil {
		return err
	}

	return rawBlockTransactions.Put(database, blockNumber, &bytes)
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

func createBatch(
	logger utils.SimpleLogger,
	database db.KeyValueStore,
	startBlock,
	endBlock uint64,
) (db.Batch, error) {
	debugLog(logger, "fetching block", startBlock, endBlock)
	batch := database.NewBatch()

	for blockNumber := startBlock; blockNumber <= endBlock; blockNumber++ {
		blockTransactions, shouldMigrate, err := readBlock(database, blockNumber)
		if err != nil {
			return nil, err
		}
		if !shouldMigrate {
			logger.Infow("skipping block", "blockNumber", blockNumber)
			continue
		}
		if err := writeBlock(batch, blockNumber, blockTransactions); err != nil {
			return nil, err
		}
	}

	debugLog(logger, "fetched block", startBlock, endBlock)

	err := deleteOldData(batch, startBlock, endBlock)
	return batch, err
}

func debugLog(logger utils.SimpleLogger, msg string, startBlock, endBlock uint64) {
	logger.Debugw(
		msg,
		"startBlock", startBlock,
		"endBlock", endBlock,
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

func migrateBlockRange(
	ctx context.Context,
	database db.KeyValueStore,
	logger utils.SimpleLogger,
	cleanup *cleanup,
	firstBlock,
	chainHeight uint64,
) []byte {
	counter := newCounter(logger, timeLogRate)

	s := stream.New().WithMaxGoroutines(batchConcurrency)
	defer s.Wait()

	for startBlock := firstBlock; startBlock <= chainHeight; startBlock += batchSize {
		select {
		case <-ctx.Done():
			return []byte{}
		default:
		}

		s.Go(func() stream.Callback {
			endBlock := min(startBlock+batchSize-1, chainHeight)
			batch, err := createBatch(logger, database, startBlock, endBlock)

			return func() {
				if err != nil {
					cleanup.AddError(err)
					return
				}

				debugLog(logger, "writing block", startBlock, endBlock)
				if err := batch.Write(); err != nil {
					cleanup.AddError(err)
					return
				}

				counter.log(batch, batchSize)

				debugLog(logger, "wrote block", startBlock, endBlock)

				cleanup.CloseBatch(batch)
			}
		})
	}

	return nil
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
	for {
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

		cleanup := newCleanup()
		state := migrateBlockRange(ctx, database, logger, &cleanup, firstBlock, chainHeight)
		err = cleanup.Wait()
		if state != nil || err != nil {
			return state, errors.Join(ctx.Err(), err)
		}
	}
}
