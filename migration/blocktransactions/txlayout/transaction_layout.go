// Package txlayout is a frozen copy of the transaction layout logic that was
// originally defined as core.TransactionLayout, introduced in
// https://github.com/NethermindEth/juno/pull/3304.
//
// Juno now exclusively uses a combined layout to store transactions and
// receipts in the database. The original TransactionLayout type was removed
// from core, but migrations and tests still need to read and write data in
// both the old (per-transaction) and new (combined) layouts.
//
// This package preserves that capability for use by migration code and tests.
// It should NOT be used by any new production code paths.
package txlayout

import (
	"errors"
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type TransactionLayout int

const (
	// TransactionLayoutPerTx stores each transaction and receipt in separate DB entries
	TransactionLayoutPerTx TransactionLayout = iota
	// TransactionLayoutCombined stores all transactions and receipts per block in a single entry
	TransactionLayoutCombined
)

var ErrUnknownTransactionLayout = errors.New("unknown transaction layout")

// TransactionByBlockAndIndex returns a transaction by block number and index
func (l TransactionLayout) TransactionByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (core.Transaction, error) {
	switch l {
	case TransactionLayoutCombined:
		return core.BlockTransactionsTransactionPartialBucket.Get(r, blockNumber, int(index))

	case TransactionLayoutPerTx:
		key := db.BlockNumIndexKey{
			Number: blockNumber,
			Index:  index,
		}
		return core.TransactionsByBlockNumberAndIndexBucket.Get(r, key)

	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// ReceiptByBlockAndIndex returns a receipt by block number and transaction index
func (l TransactionLayout) ReceiptByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (*core.TransactionReceipt, error) {
	switch l {
	case TransactionLayoutCombined:
		return core.BlockTransactionsReceiptPartialBucket.Get(r, blockNumber, int(index))

	case TransactionLayoutPerTx:
		key := db.BlockNumIndexKey{
			Number: blockNumber,
			Index:  index,
		}
		receipt, err := core.ReceiptsByBlockNumberAndIndexBucket.Get(r, key)
		if err != nil {
			return nil, err
		}
		return &receipt, nil

	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// TransactionsByBlockNumber returns all transactions in a given block
func (l TransactionLayout) TransactionsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]core.Transaction, error) {
	switch l {
	case TransactionLayoutCombined:
		return core.BlockTransactionsAllTransactionsPartialBucket.Get(r, blockNum, struct{}{})

	case TransactionLayoutPerTx:
		var transactions []core.Transaction
		iterator := core.TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
		for entry, err := range iterator {
			if err != nil {
				return nil, err
			}
			transactions = append(transactions, entry.Value)
		}
		return transactions, nil

	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// TransactionsByBlockNumberIter returns an iterator over all transactions in a given block
func (l TransactionLayout) TransactionsByBlockNumberIter(
	r db.KeyValueReader,
	blockNum uint64,
) iter.Seq2[core.Transaction, error] {
	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := core.BlockTransactionsBucket.Get(r, blockNum)
		if err != nil {
			return func(yield func(core.Transaction, error) bool) {
				yield(nil, err)
			}
		}
		return blockTransactions.Transactions().Iter()

	case TransactionLayoutPerTx:
		return func(yield func(core.Transaction, error) bool) {
			iterator := core.TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
			for entry, err := range iterator {
				if !yield(entry.Value, err) {
					return
				}
			}
		}

	default:
		return func(yield func(core.Transaction, error) bool) {
			yield(nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l))
		}
	}
}

// ReceiptsByBlockNumber returns all receipts in a given block
func (l TransactionLayout) ReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]*core.TransactionReceipt, error) {
	switch l {
	case TransactionLayoutCombined:
		return core.BlockTransactionsAllReceiptsPartialBucket.Get(r, blockNum, struct{}{})

	case TransactionLayoutPerTx:
		var receipts []*core.TransactionReceipt
		iterator := core.ReceiptsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
		for entry, err := range iterator {
			if err != nil {
				return nil, err
			}
			receipts = append(receipts, &entry.Value)
		}
		return receipts, nil

	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// BlockByNumber returns a full block by number
func (l TransactionLayout) BlockByNumber(
	r db.KeyValueReader,
	blockNum uint64,
) (*core.Block, error) {
	header, err := core.GetBlockHeaderByNumber(r, blockNum)
	if err != nil {
		return nil, err
	}

	allTransactions, err := l.TransactionsByBlockNumber(r, blockNum)
	if err != nil {
		return nil, err
	}

	allReceipts, err := l.ReceiptsByBlockNumber(r, blockNum)
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
func (l TransactionLayout) WriteTransactionsAndReceipts(
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
	}

	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := core.NewBlockTransactions(transactions, receipts)
		if err != nil {
			return err
		}

		return core.BlockTransactionsBucket.Put(w, blockNumber, &blockTransactions)

	case TransactionLayoutPerTx:
		for index, tx := range transactions {
			key := db.BlockNumIndexKey{
				Number: blockNumber,
				Index:  uint64(index),
			}

			if err := core.TransactionsByBlockNumberAndIndexBucket.Put(w, key, &tx); err != nil {
				return err
			}
			if err := core.ReceiptsByBlockNumberAndIndexBucket.Put(w, key, receipts[index]); err != nil {
				return err
			}
		}

		return nil

	default:
		return fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// DeleteTxsAndReceipts deletes all transactions and receipts for a block
func (l TransactionLayout) DeleteTxsAndReceipts(
	reader db.KeyValueReader,
	writer db.Batch,
	blockNum uint64,
) error {
	// Delete hash mappings using the existing iterator
	for tx, err := range l.TransactionsByBlockNumberIter(reader, blockNum) {
		if err != nil {
			return err
		}
		txHash := (*felt.TransactionHash)(tx.Hash())
		err := core.TransactionBlockNumbersAndIndicesByHashBucket.Delete(writer, txHash)
		if err != nil {
			return err
		}
		if l1handler, ok := tx.(*core.L1HandlerTransaction); ok {
			err := core.DeleteL1HandlerTxnHashByMsgHash(writer, l1handler.MessageHash())
			if err != nil {
				return err
			}
		}
	}

	// Layout-specific deletions
	switch l {
	case TransactionLayoutCombined:
		return core.BlockTransactionsBucket.Delete(writer, blockNum)

	case TransactionLayoutPerTx:
		err := core.TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).DeletePrefix(writer)
		if err != nil {
			return err
		}
		return core.ReceiptsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).DeletePrefix(writer)

	default:
		return fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// TransactionByHash returns a transaction by its hash
func (l TransactionLayout) TransactionByHash(
	r db.KeyValueReader,
	hash *felt.TransactionHash,
) (core.Transaction, error) {
	blockNumIndex, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(r, hash)
	if err != nil {
		return nil, err
	}

	return l.TransactionByBlockAndIndex(r, blockNumIndex.Number, blockNumIndex.Index)
}
