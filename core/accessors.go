package core

import (
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/cbor"
)

/**
State-related accessors

███████ ████████  █████  ████████ ███████
██         ██    ██   ██    ██    ██
███████    ██    ███████    ██    █████
     ██    ██    ██   ██    ██    ██
███████    ██    ██   ██    ██    ███████

**/

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*DeclaredClassDefinition, error) {
	var class *DeclaredClassDefinition

	err := r.Get(db.ClassKey(classHash), func(data []byte) error {
		return cbor.Unmarshal(data, &class)
	})
	return class, err
}

func WriteClass(w db.KeyValueWriter, classHash *felt.Felt, class *DeclaredClassDefinition) error {
	data, err := cbor.Marshal(class)
	if err != nil {
		return err
	}
	return w.Put(db.ClassKey(classHash), data)
}

func GetStateUpdateByHash(r db.KeyValueReader, hash *felt.Felt) (*StateUpdate, error) {
	blockNum, err := BlockHeaderNumbersByHashBucket.Get(r, hash)
	if err != nil {
		return nil, err
	}

	stateUpdate, err := StateUpdatesByBlockNumberBucket.Get(r, blockNum)
	if err != nil {
		return nil, err
	}

	return &stateUpdate, nil
}

/**
 Chain-related accessors

 ██████ ██   ██  █████  ██ ███    ██
██      ██   ██ ██   ██ ██ ████   ██
██      ███████ ███████ ██ ██ ██  ██
██      ██   ██ ██   ██ ██ ██  ██ ██
 ██████ ██   ██ ██   ██ ██ ██   ████

 **/

func GetL1HandlerTxnHashByMsgHash(r db.KeyValueReader, msgHash []byte) (felt.Felt, error) {
	var l1HandlerTxnHash felt.Felt
	err := r.Get(db.L1HandlerTxnHashByMsgHashKey(msgHash), func(data []byte) error {
		l1HandlerTxnHash.Unmarshal(data)
		return nil
	})
	return l1HandlerTxnHash, err
}

func WriteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte, l1HandlerTxnHash *felt.Felt) error {
	return w.Put(db.L1HandlerTxnHashByMsgHashKey(msgHash), l1HandlerTxnHash.Marshal())
}

func DeleteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte) error {
	return w.Delete(db.L1HandlerTxnHashByMsgHashKey(msgHash))
}

func WriteL1HandlerMsgHashes(w db.KeyValueWriter, txns []Transaction) error {
	for _, txn := range txns {
		if l1Handler, ok := txn.(*L1HandlerTransaction); ok {
			err := WriteL1HandlerTxnHashByMsgHash(
				w,
				l1Handler.MessageHash(),
				l1Handler.Hash(),
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetGlobalStateRootByBlockNumber only materialise GlobalStateRoot from the header,
// callers opening state don't need the rest of the header.
func GetGlobalStateRootByBlockNumber(r db.KeyValueReader, blockNum uint64) (*felt.Felt, error) {
	var header headerGlobalStateRootProjection
	err := r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return cbor.Unmarshal(data, &header)
	})
	if err != nil {
		return nil, err
	}
	if header.GlobalStateRoot == nil {
		return nil, fmt.Errorf("missing GlobalStateRoot in block header %d", blockNum)
	}
	return header.GlobalStateRoot, nil
}

// GetBlockHeaderHashByNumber only materialises Hash from the header,
// skipping heavier unused fields.
func GetBlockHeaderHashByNumber(r db.KeyValueReader, blockNum uint64) (*felt.Felt, error) {
	var header headerHashProjection
	err := r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return cbor.Unmarshal(data, &header)
	})
	if err != nil {
		return nil, err
	}
	if header.Hash == nil {
		return nil, fmt.Errorf("missing Hash in block header %d", blockNum)
	}
	return header.Hash, nil
}

// GetBlockTransactionCountByNumber decodes TransactionCount from the stored
// header, skipping allocation of the unused fields.
func GetBlockTransactionCountByNumber(r db.KeyValueReader, blockNum uint64) (uint64, error) {
	var header headerTransactionCountProjection
	err := r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return cbor.Unmarshal(data, &header)
	})
	if err != nil {
		return 0, err
	}
	return header.TransactionCount, nil
}

func GetBlockHeaderTimestampByNumber(r db.KeyValueReader, blockNum uint64) (uint64, error) {
	var header headerTimestampProjection
	err := r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return cbor.Unmarshal(data, &header)
	})
	if err != nil {
		return 0, err
	}
	if header.Timestamp == nil {
		return 0, fmt.Errorf("missing Timestamp in block header %d", blockNum)
	}
	return *header.Timestamp, nil
}

func GetBlockHeaderHashAndStateRootByNumber(
	r db.KeyValueReader,
	blockNum uint64,
) (hash, stateRoot *felt.Felt, err error) {
	var header headerHashAndStateRootProjection
	err = r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return cbor.Unmarshal(data, &header)
	})
	if err != nil {
		return nil, nil, err
	}
	if header.Hash == nil {
		return nil, nil, fmt.Errorf("missing Hash in block header %d", blockNum)
	}
	if header.GlobalStateRoot == nil {
		return nil, nil, fmt.Errorf("missing GlobalStateRoot in block header %d", blockNum)
	}
	return header.Hash, header.GlobalStateRoot, nil
}

func GetBlockHeaderByHash(r db.KeyValueReader, hash *felt.Felt) (*Header, error) {
	blockNum, err := BlockHeaderNumbersByHashBucket.Get(r, hash)
	if err != nil {
		return nil, err
	}

	header, err := BlockHeadersByNumberBucket.Get(r, blockNum)
	if err != nil {
		return nil, err
	}

	return &header, nil
}

func WriteBlockHeader(w db.KeyValueWriter, header *Header) error {
	if err := BlockHeaderNumbersByHashBucket.Put(w, header.Hash, &header.Number); err != nil {
		return err
	}

	return BlockHeadersByNumberBucket.Put(w, header.Number, header)
}

// GetTransactionsByBlockNumber returns all transactions in a given block
func GetTransactionsByBlockNumber(
	r db.KeyValueReader,
	blockNumber uint64,
) ([]Transaction, error) {
	return BlockTransactionsAllTransactionsPartialBucket.Get(r, blockNumber, struct{}{})
}

// GetTransactionsByBlockNumberIter returns an iterator over all transactions in a given block
func GetTransactionsByBlockNumberIter(
	r db.KeyValueReader,
	blockNumber uint64,
) iter.Seq2[Transaction, error] {
	blockTransactions, err := BlockTransactionsBucket.Get(r, blockNumber)
	if err != nil {
		return func(yield func(Transaction, error) bool) {
			yield(nil, err)
		}
	}
	return blockTransactions.Transactions().Iter()
}

// GetTransactionByBlockAndIndex returns a transaction by block number and index
func GetTransactionByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (Transaction, error) {
	return BlockTransactionsTransactionPartialBucket.Get(r, blockNumber, int(index))
}

// GetTransactionByHash returns a transaction by its hash
func GetTransactionByHash(
	r db.KeyValueReader,
	hash *felt.TransactionHash,
) (Transaction, error) {
	blockNumIndex, err := TransactionBlockNumbersAndIndicesByHashBucket.Get(r, hash)
	if err != nil {
		return nil, err
	}

	return GetTransactionByBlockAndIndex(r, blockNumIndex.Number, blockNumIndex.Index)
}

// GetReceiptsByBlockNumber returns all receipts in a given block
func GetReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNumber uint64,
) ([]*TransactionReceipt, error) {
	return BlockTransactionsAllReceiptsPartialBucket.Get(r, blockNumber, struct{}{})
}

// GetTransactionEventsByBlockNumber returns the events subset of every receipt in the
// block. It skips the heavier receipt fields.
func GetTransactionEventsByBlockNumber(
	r db.KeyValueReader,
	blockNumber uint64,
) ([]TransactionEvents, error) {
	return BlockTransactionsAllTransactionEventsPartialBucket.Get(r, blockNumber, struct{}{})
}

// GetTransactionHashesByBlockNumber returns transaction-hashes of given block.
func GetTransactionHashesByBlockNumber(
	r db.KeyValueReader,
	blockNumber uint64,
) ([]felt.Felt, error) {
	return BlockTransactionsAllTransactionHashesPartialBucket.Get(r, blockNumber, struct{}{})
}

// GetTransactionsAndReceiptsByBlockNumber returns all transactions and receipts in a given block.
// Both live under the same key, so this reads the block only once.
func GetTransactionsAndReceiptsByBlockNumber(
	r db.KeyValueReader,
	blockNumber uint64,
) ([]Transaction, []*TransactionReceipt, error) {
	result, err := BlockTransactionsAllTransactionsAndReceiptsPartialBucket.Get(
		r,
		blockNumber,
		struct{}{},
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"getting transactions and receipts of block %d: %w",
			blockNumber,
			err,
		)
	}

	return result.Transactions, result.Receipts, nil
}

// GetReceiptByBlockAndIndex returns a receipt by block number and transaction index
func GetReceiptByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (*TransactionReceipt, error) {
	return BlockTransactionsReceiptPartialBucket.Get(r, blockNumber, int(index))
}

// GetTransactionAndReceiptByBlockAndIndex returns a transaction and its receipt by block number
// and transaction index. Both live under the same key, so this reads the block only once.
func GetTransactionAndReceiptByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (Transaction, *TransactionReceipt, error) {
	pair, err := BlockTransactionsTransactionAndReceiptPartialBucket.Get(
		r, blockNumber, int(index),
	)
	if err != nil {
		return nil, nil, err
	}
	return pair.Transaction, pair.Receipt, nil
}

// GetTransactionExecutionStatusByBlockAndIndex returns only the status subset of a
// receipt, skipping its heavier fields.
func GetTransactionExecutionStatusByBlockAndIndex(
	r db.KeyValueReader,
	blockNumber uint64,
	index uint64,
) (TransactionExecutionStatus, error) {
	return BlockTransactionsExecutionStatusPartialBucket.Get(r, blockNumber, int(index))
}

// GetBlockByNumber returns a full block by number
func GetBlockByNumber(r db.KeyValueReader, blockNumber uint64) (*Block, error) {
	header, err := BlockHeadersByNumberBucket.Get(r, blockNumber)
	if err != nil {
		return nil, err
	}

	txs, receipts, err := GetTransactionsAndReceiptsByBlockNumber(r, blockNumber)
	if err != nil {
		return nil, err
	}

	return &Block{
		Header:       &header,
		Transactions: txs,
		Receipts:     receipts,
	}, nil
}

// WriteTransactionsAndReceipts writes transactions and receipts for a block
func WriteTransactionsAndReceipts(
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

	blockTransactions, err := NewBlockTransactions(transactions, receipts)
	if err != nil {
		return err
	}

	return BlockTransactionsBucket.Put(w, blockNumber, &blockTransactions)
}

// DeleteTransactionsAndReceipts deletes all transactions and receipts for a block
func DeleteTransactionsAndReceipts(
	reader db.KeyValueReader,
	writer db.Batch,
	blockNumber uint64,
) error {
	// Delete hash mappings using the existing iterator
	for tx, err := range GetTransactionsByBlockNumberIter(reader, blockNumber) {
		if err != nil {
			return err
		}
		txHash := (*felt.TransactionHash)(tx.Hash())
		err := TransactionBlockNumbersAndIndicesByHashBucket.Delete(writer, txHash)
		if err != nil {
			return err
		}
		if l1handler, ok := tx.(*L1HandlerTransaction); ok {
			err := DeleteL1HandlerTxnHashByMsgHash(writer, l1handler.MessageHash())
			if err != nil {
				return err
			}
		}
	}

	return BlockTransactionsBucket.Delete(writer, blockNumber)
}

func GetAggregatedBloomFilterView(
	r db.KeyValueReader,
	fromBlock, toBlock uint64,
) (AggregatedBloomFilterView, error) {
	var view AggregatedBloomFilterView
	err := r.Get(db.AggregatedBloomFilterKey(fromBlock, toBlock), func(data []byte) error {
		return cbor.Unmarshal(data, &view)
	})
	if err != nil {
		return AggregatedBloomFilterView{}, err
	}

	return view, nil
}

func GetRunningEventFilter(r db.KeyValueReader) (*RunningEventFilter, error) {
	var filter RunningEventFilter
	err := r.Get(db.RunningEventFilter.Key(), func(data []byte) error {
		err := cbor.Unmarshal(data, &filter)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &filter, nil
}

func WriteRunningEventFilter(w db.KeyValueWriter, filter *RunningEventFilter) error {
	enc, err := cbor.Marshal(filter)
	if err != nil {
		return err
	}

	return w.Put(db.RunningEventFilter.Key(), enc)
}
