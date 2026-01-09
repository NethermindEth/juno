package blocktransactions

import (
	"iter"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/prefix"
)

func getFirstBlockToMigrate(
	database db.KeyValueReader,
) (firstBlock uint64, shouldMigrate bool, err error) {
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

func getFirstBlockInBucket[A any](items iter.Seq2[prefix.Entry[A], error]) (uint64, bool, error) {
	next, stop := iter.Pull2(items)
	defer stop()

	item, err, ok := next()
	if !ok {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	var blockNumIndex db.BlockNumIndexKey
	if err := blockNumIndex.UnmarshalBinary(item.Key[1:]); err != nil {
		return 0, false, err
	}

	return blockNumIndex.Number, true, nil
}

func clearOldBuckets(database db.KeyValueStore) error {
	err := core.TransactionsByBlockNumberAndIndexBucket.Prefix().DeletePrefix(database)
	if err != nil {
		return err
	}

	return core.ReceiptsByBlockNumberAndIndexBucket.Prefix().DeletePrefix(database)
}
