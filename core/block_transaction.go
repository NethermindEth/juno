package core

import (
	"iter"

	"github.com/NethermindEth/juno/core/indexed"
)

type BlockTransactionsIndexes struct {
	Transactions []int `cbor:"1,keyasint,omitempty"`
	Receipts     []int `cbor:"2,keyasint,omitempty"`
}

// All transactions and receipts of the same block are stored in a single DB entry. This
// significantly reduces the number of DB entries. The number of entries now scales with the number
// of blocks instead of the number of transactions.
// If we simply store slices of transactions and receipts, we would need to read the entire block
// to get the transactions and receipts. This is inefficient, especially for blocks with a lot of
// transactions and receipts.
// Instead, the data consists of 2 parts. The first part is a CBOR encoded struct of 2 slices of
// indexes, one for transactions and one for receipts. The second part is a byte slice where
// transactions and receipts are stored contiguously in one byte slice, with indexes
// slices marking each item's start offset. This allows us to unmarshal any transaction or receipt
// on demand. Illustration:
//
// transactions:  [00    02          06]
// *               ↓     ↓           ↓
// *               ----- ----------- -----------
// data:          [00|01|02|03|04|05|06|07|08|09|10|11|12|13|14|15|16|17|18|19]
// *                                             ----- -------- --------------
// *                                             ↑     ↑        ↑              ↑
// receipts:                                    [10    12       15]            20 (len(data))
type BlockTransactions struct {
	Indexes BlockTransactionsIndexes
	Data    []byte
}

func NewBlockTransactionsFromIterators[T, R any](
	transactions iter.Seq2[T, error],
	receipts iter.Seq2[R, error],
) (BlockTransactions, error) {
	writer := indexed.NewBufferedEncoder()
	transactionIndexes, err := indexed.Write(writer, transactions)
	if err != nil {
		return BlockTransactions{}, err
	}

	receiptIndexes, err := indexed.Write(writer, receipts)
	if err != nil {
		return BlockTransactions{}, err
	}

	return BlockTransactions{
		Indexes: BlockTransactionsIndexes{
			Transactions: transactionIndexes,
			Receipts:     receiptIndexes,
		},
		Data: writer.Bytes(),
	}, nil
}

func wrapWithNilError[T any](items []T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for _, item := range items {
			if !yield(item, nil) {
				return
			}
		}
	}
}

func NewBlockTransactions(
	transactions []Transaction,
	receipts []*TransactionReceipt,
) (BlockTransactions, error) {
	return NewBlockTransactionsFromIterators(
		wrapWithNilError(transactions),
		wrapWithNilError(receipts),
	)
}

func (b *BlockTransactions) Transactions() indexed.LazySlice[Transaction] {
	end := len(b.Data)
	if len(b.Indexes.Receipts) > 0 {
		end = b.Indexes.Receipts[0]
	}

	return indexed.NewLazySlice[Transaction](b.Indexes.Transactions, b.Data[:end])
}

func (b *BlockTransactions) Receipts() indexed.LazySlice[*TransactionReceipt] {
	return indexed.NewLazySlice[*TransactionReceipt](b.Indexes.Receipts, b.Data)
}
