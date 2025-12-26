package core

import (
	"bytes"
	"iter"
	"slices"

	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/encoder"
)

type BlockTransactionsIndexes struct {
	Transactions []int `cbor:"1,keyasint,omitempty"`
	Receipts     []int `cbor:"2,keyasint,omitempty"`
}

type BlockTransactions struct {
	Indexes BlockTransactionsIndexes
	Data    []byte
}

func NewBlockTransactionsFromIterators(
	transactions iter.Seq2[any, error],
	receipts iter.Seq2[any, error],
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

func wrapWithNilError[T any](items []T) iter.Seq2[any, error] {
	return func(yield func(any, error) bool) {
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

type BlockTransactionsSerializer struct{}

func (BlockTransactionsSerializer) Marshal(value *BlockTransactions) ([]byte, error) {
	var buf bytes.Buffer
	if err := encoder.NewEncoder(&buf).Encode(value.Indexes); err != nil {
		return nil, err
	}
	if _, err := buf.Write(value.Data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (BlockTransactionsSerializer) Unmarshal(data []byte, value *BlockTransactions) error {
	remaining, err := encoder.UnmarshalFirst(data, &value.Indexes)
	value.Data = slices.Clone(remaining)
	return err
}
