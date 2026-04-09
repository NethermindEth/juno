// Package oldlayout contains accessors for the old transactions and receipts buckets.
// Although we now have the new transaction layout, we need these accessors in
// some of the old migrations.
package oldlayout

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// TransactionsByBlockNumber returns all transactions in a given block
// using the old transaction layout.
func TransactionsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]core.Transaction, error) {
	var transactions []core.Transaction
	iterator := core.TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
	for entry, err := range iterator {
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, entry.Value)
	}
	return transactions, nil
}

// ReceiptsByBlockNumber returns all receipts in a given block
// using the old transaction layout.
func ReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]*core.TransactionReceipt, error) {
	var receipts []*core.TransactionReceipt
	iterator := core.ReceiptsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
	for entry, err := range iterator {
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, &entry.Value)
	}
	return receipts, nil
}

// BlockByNumber returns a full block by number
// using the old transaction layout.
func BlockByNumber(r db.KeyValueReader, blockNum uint64) (*core.Block, error) {
	header, err := core.GetBlockHeaderByNumber(r, blockNum)
	if err != nil {
		return nil, err
	}

	allTransactions, err := TransactionsByBlockNumber(r, blockNum)
	if err != nil {
		return nil, err
	}

	allReceipts, err := ReceiptsByBlockNumber(r, blockNum)
	if err != nil {
		return nil, err
	}

	return &core.Block{
		Header:       header,
		Transactions: allTransactions,
		Receipts:     allReceipts,
	}, nil
}

// WriteTransactionsAndReceipts writes transactions and receipts for a block
// using the old transaction layout.
func WriteTransactionsAndReceipts(
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

		if err := core.TransactionBlockNumbersAndIndicesByHashBucket.Put(w, txHash, &key); err != nil {
			return err
		}

		if err := core.TransactionsByBlockNumberAndIndexBucket.Put(w, key, &tx); err != nil {
			return err
		}
		if err := core.ReceiptsByBlockNumberAndIndexBucket.Put(w, key, receipts[index]); err != nil {
			return err
		}
	}

	return nil
}
