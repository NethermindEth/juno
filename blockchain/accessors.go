package blockchain

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

func GetTxBlockNumIndexByHash(r db.KeyValueReader, hash *felt.Felt) (db.BlockNumIndexKey, error) {
	bnIndex := db.BlockNumIndexKey{}
	err := r.Get(db.TxBlockNumIndexByHashKey(hash), bnIndex.UnmarshalBinary)
	return bnIndex, err
}

func WriteTxBlockNumIndexByHash(w db.KeyValueWriter, num, index uint64, hash *felt.Felt) error {
	val := db.BlockNumIndexKey{
		Number: num,
		Index:  index,
	}
	return w.Put(db.TxBlockNumIndexByHashKey(hash), val.MarshalBinary())
}

func DeleteTxBlockNumIndexByHash(w db.KeyValueWriter, hash *felt.Felt) error {
	return w.Delete(db.TxBlockNumIndexByHashKey(hash))
}

func GetTxByBlockNumIndex(r db.KeyValueReader, blockNum, index uint64) (core.Transaction, error) {
	var tx core.Transaction
	err := r.Get(db.TxByBlockNumIndexKey(blockNum, index), func(data []byte) error {
		return encoder.Unmarshal(data, &tx)
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func GetTxByBlockNumIndexBytes(r db.KeyValueReader, val []byte) (core.Transaction, error) {
	var tx core.Transaction
	err := r.Get(db.TxByBlockNumIndexKeyBytes(val), func(data []byte) error {
		return encoder.Unmarshal(data, &tx)
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func WriteTxByBlockNumIndex(w db.KeyValueWriter, num, index uint64, tx core.Transaction) error {
	enc, err := encoder.Marshal(tx)
	if err != nil {
		return err
	}
	return w.Put(db.TxByBlockNumIndexKey(num, index), enc)
}

func DeleteTxByBlockNumIndex(w db.KeyValueWriter, num, index uint64) error {
	return w.Delete(db.TxByBlockNumIndexKey(num, index))
}

func WriteTxAndReceipt(w db.KeyValueWriter, num, index uint64, tx core.Transaction, receipt *core.TransactionReceipt) error {
	if err := WriteTxBlockNumIndexByHash(w, num, index, receipt.TransactionHash); err != nil {
		return err
	}

	if err := WriteTxByBlockNumIndex(w, num, index, tx); err != nil {
		return err
	}

	if err := WriteReceiptByBlockNumIndex(w, num, index, receipt); err != nil {
		return err
	}

	return nil
}

func GetTxByHash(r db.KeyValueReader, hash *felt.Felt) (core.Transaction, error) {
	var val []byte
	err := r.Get(db.TxBlockNumIndexByHashKey(hash), func(data []byte) error {
		val = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetTxByBlockNumIndexBytes(r, val)
}

func GetReceiptByBlockNumIndexBytes(r db.KeyValueReader, bnIndex []byte) (*core.TransactionReceipt, error) {
	var (
		receipt *core.TransactionReceipt
		val     []byte
	)
	err := r.Get(db.ReceiptByBlockNumIndexKeyBytes(bnIndex), func(data []byte) error {
		val = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = encoder.Unmarshal(val, &receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

func WriteReceiptByBlockNumIndex(w db.KeyValueWriter, num, index uint64, receipt *core.TransactionReceipt) error {
	data, err := encoder.Marshal(receipt)
	if err != nil {
		return err
	}
	return w.Put(db.ReceiptByBlockNumIndexKey(num, index), data)
}

func DeleteReceiptByBlockNumIndex(w db.KeyValueWriter, num, index uint64) error {
	return w.Delete(db.ReceiptByBlockNumIndexKey(num, index))
}

func GetReceiptByHash(r db.KeyValueReader, hash *felt.Felt) (*core.TransactionReceipt, error) {
	var val []byte
	err := r.Get(db.TxBlockNumIndexByHashKey(hash), func(data []byte) error {
		val = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetReceiptByBlockNumIndexBytes(r, val)
}

func DeleteTxsAndReceipts(r db.KeyValueReader, w db.KeyValueWriter, blockNum, numTxs uint64) error {
	// remove txs and receipts
	for i := range numTxs {
		reorgedTxn, err := GetTxByBlockNumIndex(r, blockNum, i)
		if err != nil {
			return err
		}

		if err := DeleteTxByBlockNumIndex(w, blockNum, i); err != nil {
			return err
		}
		if err := DeleteReceiptByBlockNumIndex(w, blockNum, i); err != nil {
			return err
		}
		if err := DeleteTxBlockNumIndexByHash(w, reorgedTxn.Hash()); err != nil {
			return err
		}
		if l1handler, ok := reorgedTxn.(*core.L1HandlerTransaction); ok {
			if err := DeleteL1HandlerTxnHashByMsgHash(w, l1handler.MessageHash()); err != nil {
				return err
			}
		}
	}

	return nil
}

func GetStateUpdateByBlockNum(r db.KeyValueReader, blockNum uint64) (*core.StateUpdate, error) {
	var stateUpdate *core.StateUpdate
	err := r.Get(db.StateUpdateByBlockNumKey(blockNum), func(data []byte) error {
		return encoder.Unmarshal(data, &stateUpdate)
	})
	if err != nil {
		return nil, err
	}
	return stateUpdate, nil
}

func WriteStateUpdateByBlockNum(w db.KeyValueWriter, blockNum uint64, stateUpdate *core.StateUpdate) error {
	data, err := encoder.Marshal(stateUpdate)
	if err != nil {
		return err
	}
	return w.Put(db.StateUpdateByBlockNumKey(blockNum), data)
}

func DeleteStateUpdateByBlockNum(w db.KeyValueWriter, blockNum uint64) error {
	return w.Delete(db.StateUpdateByBlockNumKey(blockNum))
}

func GetStateUpdateByHash(r db.KeyValueReader, hash *felt.Felt) (*core.StateUpdate, error) {
	var val []byte
	err := r.Get(db.BlockHeaderNumbersByHashKey(hash), func(data []byte) error {
		val = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetStateUpdateByBlockNum(r, binary.BigEndian.Uint64(val))
}

func GetBlockCommitmentByBlockNum(r db.KeyValueReader, blockNum uint64) (*core.BlockCommitments, error) {
	var commitment *core.BlockCommitments
	err := r.Get(db.BlockCommitmentsKey(blockNum), func(data []byte) error {
		return encoder.Unmarshal(data, &commitment)
	})
	return commitment, err
}

func WriteBlockCommitment(w db.KeyValueWriter, blockNum uint64, commitment *core.BlockCommitments) error {
	data, err := encoder.Marshal(commitment)
	if err != nil {
		return err
	}
	return w.Put(db.BlockCommitmentsKey(blockNum), data)
}

func DeleteBlockCommitment(w db.KeyValueWriter, blockNum uint64) error {
	return w.Delete(db.BlockCommitmentsKey(blockNum))
}

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

func WriteL1HandlerMsgHashes(w db.KeyValueWriter, txns []core.Transaction) error {
	for _, txn := range txns {
		if l1Handler, ok := txn.(*core.L1HandlerTransaction); ok {
			return WriteL1HandlerTxnHashByMsgHash(w, l1Handler.MessageHash(), l1Handler.Hash())
		}
	}
	return nil
}

func GetChainHeight(r db.KeyValueReader) (uint64, error) {
	var height uint64
	err := r.Get(db.ChainHeight.Key(), func(data []byte) error {
		height = binary.BigEndian.Uint64(data)
		return nil
	})
	return height, err
}

func WriteChainHeight(w db.KeyValueWriter, height uint64) error {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], height)
	return w.Put(db.ChainHeight.Key(), b[:])
}

func DeleteChainHeight(w db.KeyValueWriter) error {
	return w.Delete(db.ChainHeight.Key())
}

func GetBlockHeaderByNumber(r db.KeyValueReader, blockNum uint64) (*core.Header, error) {
	var header *core.Header
	err := r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return encoder.Unmarshal(data, &header)
	})
	return header, err
}

func GetBlockHeaderByHash(r db.KeyValueReader, hash *felt.Felt) (*core.Header, error) {
	var blockNum uint64
	err := r.Get(db.BlockHeaderNumbersByHashKey(hash), func(data []byte) error {
		blockNum = binary.BigEndian.Uint64(data)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(r, blockNum)
}

func WriteBlockHeaderByNumber(r db.KeyValueWriter, header *core.Header) error {
	enc, err := encoder.Marshal(header)
	if err != nil {
		return err
	}

	return r.Put(db.BlockHeaderByNumberKey(header.Number), enc)
}

func WriteBlockHeaderNumberByHash(r db.KeyValueWriter, hash *felt.Felt, num uint64) error {
	numBytes := core.MarshalBlockNumber(num)
	return r.Put(db.BlockHeaderNumbersByHashKey(hash), numBytes)
}

func GetBlockHeaderNumberByHash(r db.KeyValueReader, hash *felt.Felt) (uint64, error) {
	var blockNum uint64
	err := r.Get(db.BlockHeaderNumbersByHashKey(hash), func(data []byte) error {
		blockNum = binary.BigEndian.Uint64(data)
		return nil
	})
	return blockNum, err
}

func WriteBlockHeader(r db.KeyValueWriter, header *core.Header) error {
	if err := WriteBlockHeaderNumberByHash(r, header.Hash, header.Number); err != nil {
		return err
	}

	return WriteBlockHeaderByNumber(r, header)
}

// Returns all transactions in a given block
func GetTxsByBlockNum(r db.Iterable, blockNum uint64) ([]core.Transaction, error) {
	prefix := db.TransactionsByBlockNumberAndIndex.Key(core.MarshalBlockNumber(blockNum))

	it, err := r.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}

	var txs []core.Transaction
	for it.First(); it.Valid(); it.Next() {
		val, vErr := it.Value()
		if vErr != nil {
			return nil, utils.RunAndWrapOnError(it.Close, vErr)
		}

		var tx core.Transaction
		if err = encoder.Unmarshal(val, &tx); err != nil {
			return nil, utils.RunAndWrapOnError(it.Close, err)
		}

		txs = append(txs, tx)
	}

	if err = it.Close(); err != nil {
		return nil, err
	}

	return txs, nil
}

// Returns all receipts in a given block
func GetReceiptsByBlockNum(r db.Iterable, blockNum uint64) ([]*core.TransactionReceipt, error) {
	prefix := db.ReceiptsByBlockNumberAndIndex.Key(core.MarshalBlockNumber(blockNum))

	it, err := r.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}

	var receipts []*core.TransactionReceipt
	for it.First(); it.Valid(); it.Next() {
		val, vErr := it.Value()
		if vErr != nil {
			return nil, utils.RunAndWrapOnError(it.Close, vErr)
		}

		var receipt *core.TransactionReceipt
		if err = encoder.Unmarshal(val, &receipt); err != nil {
			return nil, utils.RunAndWrapOnError(it.Close, err)
		}

		receipts = append(receipts, receipt)
	}

	if err = it.Close(); err != nil {
		return nil, err
	}

	return receipts, nil
}

func GetBlockByNumber(r db.IndexedBatch, blockNum uint64) (*core.Block, error) {
	header, err := GetBlockHeaderByNumber(r, blockNum)
	if err != nil {
		return nil, err
	}

	txs, err := GetTxsByBlockNum(r, blockNum)
	if err != nil {
		return nil, err
	}

	receipts, err := GetReceiptsByBlockNum(r, blockNum)
	if err != nil {
		return nil, err
	}

	return &core.Block{
		Header:       header,
		Transactions: txs,
		Receipts:     receipts,
	}, nil
}
