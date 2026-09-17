package core

import (
	"fmt"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/v1"
)

type BlockTransactionsSerializer struct{}

func (BlockTransactionsSerializer) Marshal(value *BlockTransactions) ([]byte, error) {
	indexes, err := cbor.Marshal(value.Indexes)
	if err != nil {
		return nil, err
	}
	return slices.Concat(indexes, value.Data), nil
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
	hashes, err := b.transactionHashProjections().AllMapped(
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

type extractAllTransactionsAndReceipts struct{}

// extract decodes both halves of the entry in one pass, so reading them together needs neither a
// second lookup nor a copy of the whole block blob.
func (extractAllTransactionsAndReceipts) extract(
	b *BlockTransactions,
	_ struct{},
) (TransactionsAndReceipts, error) {
	transactions, err := b.Transactions().All()
	if err != nil {
		return TransactionsAndReceipts{}, fmt.Errorf("extracting transactions: %w", err)
	}

	receipts, err := b.Receipts().All()
	if err != nil {
		return TransactionsAndReceipts{}, fmt.Errorf("extracting receipts: %w", err)
	}

	return TransactionsAndReceipts{Transactions: transactions, Receipts: receipts}, nil
}

type extractAllTransactionEvents struct{}

func (extractAllTransactionEvents) extract(
	b *BlockTransactions,
	_ struct{},
) ([]TransactionEvents, error) {
	events, err := b.transactionEventsProjections().AllMapped(
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

//nolint:unparam // signature fixed by blockTransactionsExtractor interface
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
	remaining, err := cbor.UnmarshalFirst(data, &blockTransactions.Indexes)
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
	BlockTransactionsAllTransactionsAndReceiptsPartialSerializer = blockTransactionsPartialSerializer[
		extractAllTransactionsAndReceipts,
		struct{},
		TransactionsAndReceipts,
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
