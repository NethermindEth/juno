package core

import (
	"bytes"
	"slices"

	"github.com/NethermindEth/juno/encoder"
)

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
	partial := blockTransactionsPartialSerializer[extractAll, struct{}, BlockTransactions]{}
	return partial.UnmarshalPartial(struct{}{}, data, value)
}

type blockTransactionsExtractor[S, T any] interface {
	~struct{}
	extract(*BlockTransactions, S) (T, error)
}

type extractTransaction struct{}

func (extractTransaction) extract(b *BlockTransactions, subKey int) (Transaction, error) {
	return b.Transactions().Get(subKey)
}

type extractReceipt struct{}

func (extractReceipt) extract(b *BlockTransactions, subKey int) (*TransactionReceipt, error) {
	return b.Receipts().Get(subKey)
}

type extractAllTransactions struct{}

func (extractAllTransactions) extract(b *BlockTransactions, _ struct{}) ([]Transaction, error) {
	return b.Transactions().All()
}

type extractAllReceipts struct{}

func (extractAllReceipts) extract(b *BlockTransactions, _ struct{}) ([]*TransactionReceipt, error) {
	return b.Receipts().All()
}

type extractAll struct{}

func (extractAll) extract(b *BlockTransactions, _ struct{}) (BlockTransactions, error) {
	return BlockTransactions{
		Indexes: b.Indexes,
		Data:    slices.Clone(b.Data),
	}, nil
}

type blockTransactionsPartialSerializer[E blockTransactionsExtractor[S, T], S, T any] struct{}

func (blockTransactionsPartialSerializer[E, S, T]) UnmarshalPartial(
	subKey S,
	data []byte,
	value *T,
) error {
	var blockTransactions BlockTransactions
	remaining, err := encoder.UnmarshalFirst(data, &blockTransactions.Indexes)
	if err != nil {
		return err
	}
	blockTransactions.Data = remaining
	*value, err = E{}.extract(&blockTransactions, subKey)
	return err
}

var (
	BlockTransactionsTransactionPartialSerializer = blockTransactionsPartialSerializer[
		extractTransaction,
		int,
		Transaction,
	]{}
	BlockTransactionsReceiptPartialSerializer = blockTransactionsPartialSerializer[
		extractReceipt,
		int,
		*TransactionReceipt,
	]{}
	BlockTransactionsAllTransactionsPartialSerializer = blockTransactionsPartialSerializer[
		extractAllTransactions,
		struct{},
		[]Transaction,
	]{}
	BlockTransactionsAllReceiptsPartialSerializer = blockTransactionsPartialSerializer[
		extractAllReceipts,
		struct{},
		[]*TransactionReceipt,
	]{}
)
