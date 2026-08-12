package core

import (
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/encoder"
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

// TransactionAndReceipt is a transaction paired with its receipt.
type TransactionAndReceipt struct {
	Transaction Transaction
	Receipt     *TransactionReceipt
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

// transactionsSection is the data region holding transactions, ending where receipts begin.
func (b *BlockTransactions) transactionsSection() []byte {
	if len(b.Indexes.Receipts) > 0 {
		return b.Data[:b.Indexes.Receipts[0]]
	}
	return b.Data
}

// receiptsSection is the data region holding receipts, which runs to the end of the data.
func (b *BlockTransactions) receiptsSection() []byte {
	return b.Data
}

func (b *BlockTransactions) Transactions() indexed.LazySlice[Transaction] {
	return indexed.NewLazySlice[Transaction](b.Indexes.Transactions, b.transactionsSection())
}

func (b *BlockTransactions) Receipts() indexed.LazySlice[*TransactionReceipt] {
	return indexed.NewLazySlice[*TransactionReceipt](b.Indexes.Receipts, b.receiptsSection())
}

// executionStatusProjectionSlice is the lazily-decoded slice of execution-status projections.
type executionStatusProjectionSlice = indexed.LazySlice[receiptExecutionStatusProjection]

// executionStatusProjections decodes receipts into the execution-status subset, skipping the
// heavier receipt fields.
func (b *BlockTransactions) executionStatusProjections() executionStatusProjectionSlice {
	return indexed.NewLazySlice[receiptExecutionStatusProjection](b.Indexes.Receipts, b.receiptsSection())
}

// transactionEventsProjectionSlice is the lazily-decoded slice of events projections.
type transactionEventsProjectionSlice = indexed.LazySlice[receiptEventsProjection]

// transactionEventsProjections decodes receipts into the events subset only.
func (b *BlockTransactions) transactionEventsProjections() transactionEventsProjectionSlice {
	return indexed.NewLazySlice[receiptEventsProjection](b.Indexes.Receipts, b.Data)
}

// TransactionHashes decodes the transaction-hash field of each transaction into one contiguous
// slice, skipping the heavier transaction fields.
func (b *BlockTransactions) TransactionHashes() ([]felt.Felt, error) {
	offsets := b.Indexes.Transactions
	section := b.transactionsSection()

	hashes := make([]felt.Felt, len(offsets))
	var projection transactionHashProjection
	for i := range offsets {
		end := len(section)
		if i < len(offsets)-1 {
			end = offsets[i+1]
		}
		// Reset before decoding: CBOR null decodes into a value-typed felt as a no-op, so a
		// transaction stored without a hash would otherwise yield the previous one's hash.
		projection.TransactionHash = felt.Zero
		if err := encoder.Unmarshal(section[offsets[i]:end], &projection); err != nil {
			return nil, fmt.Errorf("decoding transact hash projection at index %d: %w", i, err)
		}
		if projection.TransactionHash.IsZero() {
			return nil, fmt.Errorf("missing TransactionHash in transaction %d", i)
		}
		hashes[i] = projection.TransactionHash
	}
	return hashes, nil
}
