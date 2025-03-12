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
	var bnIndex db.BlockNumIndexKey
	data, err := r.Get2(db.TxBlockNumIndexByHashKey(hash))
	if err != nil {
		return db.BlockNumIndexKey{}, err
	}

	err = bnIndex.UnmarshalBinary(data)
	if err != nil {
		return db.BlockNumIndexKey{}, err
	}

	return bnIndex, nil
}

func WriteTxBlockNumIndexByHash(w db.KeyValueWriter, hash *felt.Felt, bnIndex *txAndReceiptDBKey) error {
	return w.Put(db.TxBlockNumIndexByHashKey(hash), bnIndex.MarshalBinary())
}

func DeleteTxBlockNumIndexByHash(w db.KeyValueWriter, hash *felt.Felt) error {
	return w.Delete(db.TxBlockNumIndexByHashKey(hash))
}

func GetTxByBlockNumIndex(r db.KeyValueReader, blockNum, index uint64) (core.Transaction, error) {
	var tx core.Transaction
	data, err := r.Get2(db.TxByBlockNumIndexKey(blockNum, index))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func GetTxByBlockNumIndexBytes(r db.KeyValueReader, val []byte) (core.Transaction, error) {
	var tx core.Transaction

	data, err := r.Get2(db.TxByBlockNumIndexKeyBytes(val))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &tx)
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

func GetTxByHash(r db.KeyValueReader, hash *felt.Felt) (core.Transaction, error) {
	data, err := r.Get2(db.TxBlockNumIndexByHashKey(hash))
	if err != nil {
		return nil, err
	}

	return GetTxByBlockNumIndexBytes(r, data)
}

func GetReceiptByBlockNumIndexBytes(r db.KeyValueReader, bnIndex []byte) (*core.TransactionReceipt, error) {
	var receipt *core.TransactionReceipt
	data, err := r.Get2(db.ReceiptByBlockNumIndexKeyBytes(bnIndex))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &receipt)
	if err != nil {
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
	bnIndex, err := r.Get2(db.TxBlockNumIndexByHashKey(hash))
	if err != nil {
		return nil, err
	}

	return GetReceiptByBlockNumIndexBytes(r, bnIndex)
}

func GetStateUpdateByBlockNum(r db.KeyValueReader, blockNum uint64) (*core.StateUpdate, error) {
	var stateUpdate *core.StateUpdate
	data, err := r.Get2(db.StateUpdateByBlockNumKey(blockNum))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &stateUpdate)
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
	blockNum, err := r.Get2(db.BlockHeaderNumbersByHashKey(hash))
	if err != nil {
		return nil, err
	}

	return GetStateUpdateByBlockNum(r, binary.BigEndian.Uint64(blockNum))
}

func GetBlockCommitment(r db.KeyValueReader, blockNum uint64) (*core.BlockCommitments, error) {
	var commitment *core.BlockCommitments
	data, err := r.Get2(db.BlockCommitmentsKey(blockNum))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &commitment)
	if err != nil {
		return nil, err
	}
	return commitment, nil
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
	data, err := r.Get2(db.L1HandlerTxnHashByMsgHashKey(msgHash))
	if err != nil {
		return felt.Zero, err
	}
	l1HandlerTxnHash.Unmarshal(data)
	return l1HandlerTxnHash, nil
}

func WriteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte, l1HandlerTxnHash *felt.Felt) error {
	return w.Put(db.L1HandlerTxnHashByMsgHashKey(msgHash), l1HandlerTxnHash.Marshal())
}

func DeleteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte) error {
	return w.Delete(db.L1HandlerTxnHashByMsgHashKey(msgHash))
}

func GetChainHeight(r db.KeyValueReader) (uint64, error) {
	data, err := r.Get2(db.ChainHeight.Key())
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data), nil
}

func GetBlockHeaderByNumber(r db.KeyValueReader, blockNum uint64) (*core.Header, error) {
	var header *core.Header
	data, err := r.Get2(db.BlockHeaderByNumberKey(blockNum))
	if err != nil {
		return nil, err
	}

	err = encoder.Unmarshal(data, &header)
	if err != nil {
		return nil, err
	}
	return header, nil
}

func GetBlockHeaderByHash(r db.KeyValueReader, hash *felt.Felt) (*core.Header, error) {
	blockNum, err := r.Get2(db.BlockHeaderNumbersByHashKey(hash))
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(r, binary.BigEndian.Uint64(blockNum))
}

func GetBlockHeaderNumberByHash(r db.KeyValueReader, hash *felt.Felt) (uint64, error) {
	blockNum, err := r.Get2(db.BlockHeaderNumbersByHashKey(hash))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(blockNum), nil
}

// Return all transactions given a block number
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

func GetBlockByNumber(r db.KeyValueStore, blockNum uint64) (*core.Block, error) {
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
