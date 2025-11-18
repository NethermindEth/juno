package core

import (
	"bytes"
	"iter"
	"slices"

	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/core/lazy"
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

func wrapWithNilError[T any](iter iter.Seq[T]) iter.Seq2[any, error] {
	return func(yield func(any, error) bool) {
		for item := range iter {
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
	bufferedEncoder := indexed.NewBufferedEncoder()
	transactionIndexes, err := indexed.Write(
		bufferedEncoder,
		wrapWithNilError(slices.Values(transactions)),
		len(transactions),
	)
	if err != nil {
		return BlockTransactions{}, err
	}

	receiptIndexes, err := indexed.Write(
		bufferedEncoder,
		wrapWithNilError(slices.Values(receipts)),
		len(receipts),
	)
	if err != nil {
		return BlockTransactions{}, err
	}

	return BlockTransactions{
		Indexes: BlockTransactionsIndexes{
			Transactions: transactionIndexes,
			Receipts:     receiptIndexes,
		},
		Data: bufferedEncoder.Bytes(),
	}, nil
}

func (e *BlockTransactions) Transactions() lazy.Slice[Transaction] {
	end := len(e.Data)
	if len(e.Indexes.Receipts) > 0 {
		end = e.Indexes.Receipts[0]
	}

	return lazy.NewSlice[Transaction](e.Indexes.Transactions, end, e.Data)
}

func (e *BlockTransactions) Receipts() lazy.Slice[*TransactionReceipt] {
	return lazy.NewSlice[*TransactionReceipt](e.Indexes.Receipts, len(e.Data), e.Data)
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
