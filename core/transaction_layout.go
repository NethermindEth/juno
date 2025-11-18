package core

import (
	"errors"
	"fmt"
	"iter"

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
) (Transaction, error) {
	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := BlockTransactionsBucket.Get(r, blockNumber)
		if err != nil {
			return nil, err
		}
		return blockTransactions.Transactions().Get(int(index))

	case TransactionLayoutPerTx:
		key := db.BlockNumIndexKey{
			Number: blockNumber,
			Index:  index,
		}
		return TransactionsByBlockNumberAndIndexBucket.Get(r, key)

	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// ReceiptByBlockAndIndex returns a receipt by block number and transaction index
func (l TransactionLayout) ReceiptByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (*TransactionReceipt, error) {
	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := BlockTransactionsBucket.Get(r, blockNumber)
		if err != nil {
			return nil, err
		}
		return blockTransactions.Receipts().Get(int(index))

	case TransactionLayoutPerTx:
		key := db.BlockNumIndexKey{
			Number: blockNumber,
			Index:  index,
		}
		receipt, err := ReceiptsByBlockNumberAndIndexBucket.Get(r, key)
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
) ([]Transaction, error) {
	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := BlockTransactionsBucket.Get(r, blockNum)
		if err != nil {
			return nil, err
		}
		return blockTransactions.Transactions().All()

	case TransactionLayoutPerTx:
		var transactions []Transaction
		iterator := TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
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
) iter.Seq2[Transaction, error] {
	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := BlockTransactionsBucket.Get(r, blockNum)
		if err != nil {
			return func(yield func(Transaction, error) bool) {
				yield(nil, err)
			}
		}
		return blockTransactions.Transactions().Iter()

	case TransactionLayoutPerTx:
		return func(yield func(Transaction, error) bool) {
			iterator := TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
			for entry, err := range iterator {
				if !yield(entry.Value, err) {
					return
				}
			}
		}

	default:
		return func(yield func(Transaction, error) bool) {
			yield(nil, fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l))
		}
	}
}

// ReceiptsByBlockNumber returns all receipts in a given block
func (l TransactionLayout) ReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNum uint64,
) ([]*TransactionReceipt, error) {
	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := BlockTransactionsBucket.Get(r, blockNum)
		if err != nil {
			return nil, err
		}
		return blockTransactions.Receipts().All()

	case TransactionLayoutPerTx:
		var receipts []*TransactionReceipt
		iterator := ReceiptsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).Scan(r)
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
func (l TransactionLayout) BlockByNumber(r db.KeyValueReader, blockNum uint64) (*Block, error) {
	header, err := GetBlockHeaderByNumber(r, blockNum)
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

	return &Block{
		Header:       header,
		Transactions: allTransactions,
		Receipts:     allReceipts,
	}, nil
}

// WriteTransactionsAndReceipts writes transactions and receipts for a block
func (l TransactionLayout) WriteTransactionsAndReceipts(
	w db.KeyValueWriter,
	blockNumber uint64,
	transactions []Transaction,
	receipts []*TransactionReceipt,
) error {
	for index, tx := range transactions {
		txHash := (*felt.TransactionHash)(tx.Hash())
		key := db.BlockNumIndexKey{
			Number: blockNumber,
			Index:  uint64(index),
		}

		if err := TransactionBlockNumbersAndIndicesByHashBucket.Put(w, txHash, &key); err != nil {
			return err
		}
	}

	switch l {
	case TransactionLayoutCombined:
		blockTransactions, err := NewBlockTransactions(transactions, receipts)
		if err != nil {
			return err
		}

		return BlockTransactionsBucket.Put(w, blockNumber, &blockTransactions)

	case TransactionLayoutPerTx:
		for index, tx := range transactions {
			key := db.BlockNumIndexKey{
				Number: blockNumber,
				Index:  uint64(index),
			}

			if err := TransactionsByBlockNumberAndIndexBucket.Put(w, key, &tx); err != nil {
				return err
			}
			if err := ReceiptsByBlockNumberAndIndexBucket.Put(w, key, receipts[index]); err != nil {
				return err
			}
		}

		return nil

	default:
		return fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// DeleteTxsAndReceipts deletes all transactions and receipts for a block
func (l TransactionLayout) DeleteTxsAndReceipts(batch db.IndexedBatch, blockNum uint64) error {
	// Delete hash mappings using the existing iterator
	for tx, err := range l.TransactionsByBlockNumberIter(batch, blockNum) {
		if err != nil {
			return err
		}
		txHash := (*felt.TransactionHash)(tx.Hash())
		if err := TransactionBlockNumbersAndIndicesByHashBucket.Delete(batch, txHash); err != nil {
			return err
		}
		if l1handler, ok := tx.(*L1HandlerTransaction); ok {
			if err := DeleteL1HandlerTxnHashByMsgHash(batch, l1handler.MessageHash()); err != nil {
				return err
			}
		}
	}

	// Layout-specific deletions
	switch l {
	case TransactionLayoutCombined:
		return BlockTransactionsBucket.Delete(batch, blockNum)

	case TransactionLayoutPerTx:
		err := TransactionsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).DeletePrefix(batch)
		if err != nil {
			return err
		}
		return ReceiptsByBlockNumberAndIndexBucket.Prefix().Add(blockNum).DeletePrefix(batch)

	default:
		return fmt.Errorf("%w: %d", ErrUnknownTransactionLayout, l)
	}
}

// TransactionByHash returns a transaction by its hash
func (l TransactionLayout) TransactionByHash(
	r db.KeyValueReader,
	hash *felt.TransactionHash,
) (Transaction, error) {
	blockNumIndex, err := TransactionBlockNumbersAndIndicesByHashBucket.Get(r, hash)
	if err != nil {
		return nil, err
	}

	return l.TransactionByBlockAndIndex(r, blockNumIndex.Number, blockNumIndex.Index)
}
