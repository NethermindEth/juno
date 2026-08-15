package core

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/indexed"
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

type extractTransactionAndReceipt struct{}

func (extractTransactionAndReceipt) extract(
	b *BlockTransactions,
	subKey int,
) (TransactionAndReceipt, error) {
	transaction, err := b.Transactions().Get(subKey)
	if err != nil {
		return TransactionAndReceipt{}, fmt.Errorf("extracting transaction %d: %w", subKey, err)
	}

	receipt, err := b.Receipts().Get(subKey)
	if err != nil {
		return TransactionAndReceipt{}, fmt.Errorf("extracting receipt %d: %w", subKey, err)
	}

	return TransactionAndReceipt{Transaction: transaction, Receipt: receipt}, nil
}

type extractExecutionStatus struct{}

func (extractExecutionStatus) extract(
	b *BlockTransactions,
	subKey int,
) (TransactionExecutionStatus, error) {
	projection, err := b.executionStatusProjections().Get(subKey)
	if err != nil {
		return TransactionExecutionStatus{}, err
	}
	return TransactionExecutionStatus{
		Reverted:     projection.Reverted,
		RevertReason: projection.RevertReason,
	}, nil
}

type extractAllTransactionHashes struct{}

// extract reads each transaction's own TransactionHash field, decoded without the rest of the
// transaction.
func (extractAllTransactionHashes) extract(b *BlockTransactions, _ struct{}) ([]felt.Felt, error) {
	hashes, err := indexed.AllMapped(
		b.transactionHashProjections(),
		func(i int, p transactionHashProjection) (felt.Felt, error) {
			if p.TransactionHash.IsZero() {
				return felt.Felt{}, fmt.Errorf("missing TransactionHash in transaction %d", i)
			}
			return p.TransactionHash, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("extracting transaction hashes: %w", err)
	}
	return hashes, nil
}

type extractAllTransactions struct{}

func (extractAllTransactions) extract(b *BlockTransactions, _ struct{}) ([]Transaction, error) {
	return b.Transactions().All()
}

type extractAllReceipts struct{}

func (extractAllReceipts) extract(b *BlockTransactions, _ struct{}) ([]*TransactionReceipt, error) {
	return b.Receipts().All()
}

type extractAllTransactionEvents struct{}

func (extractAllTransactionEvents) extract(
	b *BlockTransactions,
	_ struct{},
) ([]TransactionEvents, error) {
	events, err := indexed.AllMapped(
		b.transactionEventsProjections(),
		func(_ int, p receiptEventsProjection) (TransactionEvents, error) {
			return TransactionEvents{
				Events:          p.Events,
				TransactionHash: p.TransactionHash,
			}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("extracting transaction events: %w", err)
	}
	return events, nil
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
	BlockTransactionsTransactionAndReceiptPartialSerializer = blockTransactionsPartialSerializer[
		extractTransactionAndReceipt,
		int,
		TransactionAndReceipt,
	]{}
	BlockTransactionsExecutionStatusPartialSerializer = blockTransactionsPartialSerializer[
		extractExecutionStatus,
		int,
		TransactionExecutionStatus,
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
	BlockTransactionsAllTransactionEventsPartialSerializer = blockTransactionsPartialSerializer[
		extractAllTransactionEvents,
		struct{},
		[]TransactionEvents,
	]{}
	BlockTransactionsAllTransactionHashesPartialSerializer = blockTransactionsPartialSerializer[
		extractAllTransactionHashes,
		struct{},
		[]felt.Felt,
	]{}
)
