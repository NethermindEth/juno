package blockchain

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

func GetTxBlockNumAndIndexByHash(r db.KeyValueReader, hash *felt.Felt) (*txAndReceiptDBKey, error) {
	var bnIndex *txAndReceiptDBKey
	data, err := r.Get(db.TransactionBlockNumAndIndexByHashKey(hash))
	if err != nil {
		return nil, err
	}

	err = bnIndex.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}

	return bnIndex, nil
}

func WriteTxBlockNumAndIndexByHash(w db.KeyValueWriter, hash *felt.Felt, bnIndex *txAndReceiptDBKey) error {
	return w.Put(db.TransactionBlockNumAndIndexByHashKey(hash), bnIndex.MarshalBinary())
}

func DeleteTxBlockNumAndIndexByHash(w db.KeyValueWriter, hash *felt.Felt) error {
	return w.Delete(db.TransactionBlockNumAndIndexByHashKey(hash))
}

func WriteTxByBlockNumAndIndex(w db.KeyValueWriter, bnIndex []byte, tx core.Transaction) error {
	data, err := encoder.Marshal(tx)
	if err != nil {
		return err
	}
	return w.Put(db.TransactionByBlockNumAndIndexKey(bnIndex), data)
}

func GetTxByBlockNumAndIndex(r db.KeyValueReader, bnIndex *txAndReceiptDBKey) (core.Transaction, error) {
	var tx core.Transaction
	data, err := r.Get(db.TransactionByBlockNumAndIndexKey(bnIndex.MarshalBinary()))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func DeleteTxByBlockNumAndIndex(w db.KeyValueWriter, bnIndex *txAndReceiptDBKey) error {
	return w.Delete(db.TransactionByBlockNumAndIndexKey(bnIndex.MarshalBinary()))
}

func GetTxByHash(r db.KeyValueReader, hash *felt.Felt) (core.Transaction, error) {
	data, err := r.Get(db.TransactionBlockNumAndIndexByHashKey(hash))
	if err != nil {
		return nil, err
	}

	var bnIndex *txAndReceiptDBKey
	err = bnIndex.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}

	return GetTxByBlockNumAndIndex(r, bnIndex)
}

func WriteReceiptByBlockNumAndIndex(w db.KeyValueWriter, bnIndex []byte, receipt *core.TransactionReceipt) error {
	data, err := encoder.Marshal(receipt)
	if err != nil {
		return err
	}
	return w.Put(db.ReceiptByBlockNumAndIndexKey(bnIndex), data)
}

func GetReceiptByBlockNumAndIndex(r db.KeyValueReader, bnIndex *txAndReceiptDBKey) (*core.TransactionReceipt, error) {
	var receipt *core.TransactionReceipt
	data, err := r.Get(db.ReceiptByBlockNumAndIndexKey(bnIndex.MarshalBinary()))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &receipt)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func DeleteReceiptByBlockNumAndIndex(w db.KeyValueWriter, bnIndex []byte) error {
	return w.Delete(db.ReceiptByBlockNumAndIndexKey(bnIndex))
}

func GetReceiptByHash(r db.KeyValueReader, hash *felt.Felt) (*core.TransactionReceipt, error) {
	data, err := r.Get(db.TransactionBlockNumAndIndexByHashKey(hash))
	if err != nil {
		return nil, err
	}

	var bnIndex *txAndReceiptDBKey
	err = bnIndex.UnmarshalBinary(data)
	if err != nil {
		return nil, err
	}

	return GetReceiptByBlockNumAndIndex(r, bnIndex)
}

func WriteStateUpdateByBlockNum(w db.KeyValueWriter, blockNum uint64, stateUpdate *core.StateUpdate) error {
	data, err := encoder.Marshal(stateUpdate)
	if err != nil {
		return err
	}
	return w.Put(db.StateUpdateByBlockNumKey(blockNum), data)
}

func GetStateUpdateByBlockNum(r db.KeyValueReader, blockNum uint64) (*core.StateUpdate, error) {
	var stateUpdate *core.StateUpdate
	data, err := r.Get(db.StateUpdateByBlockNumKey(blockNum))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &stateUpdate)
	if err != nil {
		return nil, err
	}
	return stateUpdate, nil
}

func DeleteStateUpdateByBlockNum(w db.KeyValueWriter, blockNum uint64) error {
	return w.Delete(db.StateUpdateByBlockNumKey(blockNum))
}

func GetStateUpdateByHash(r db.KeyValueReader, hash *felt.Felt) (*core.StateUpdate, error) {
	data, err := r.Get(db.BlockHeaderNumbersByHashKey(hash))
	if err != nil {
		return nil, err
	}

	return GetStateUpdateByBlockNum(r, binary.BigEndian.Uint64(data))
}

func WriteBlockCommitment(w db.KeyValueWriter, blockNum uint64, commitment *core.BlockCommitments) error {
	data, err := encoder.Marshal(commitment)
	if err != nil {
		return err
	}
	return w.Put(db.BlockCommitmentsKey(blockNum), data)
}

func GetBlockCommitment(r db.KeyValueReader, blockNum uint64) (*core.BlockCommitments, error) {
	var commitment *core.BlockCommitments
	data, err := r.Get(db.BlockCommitmentsKey(blockNum))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &commitment)
	if err != nil {
		return nil, err
	}
	return commitment, nil
}

func DeleteBlockCommitment(w db.KeyValueWriter, blockNum uint64) error {
	return w.Delete(db.BlockCommitmentsKey(blockNum))
}

func WriteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte, l1HandlerTxnHash *felt.Felt) error {
	return w.Put(db.L1HandlerTxnHashByMsgHashKey(msgHash), l1HandlerTxnHash.Marshal())
}

func GetL1HandlerTxnHashByMsgHash(r db.KeyValueReader, msgHash []byte) (felt.Felt, error) {
	var l1HandlerTxnHash felt.Felt
	data, err := r.Get(db.L1HandlerTxnHashByMsgHashKey(msgHash))
	if err != nil {
		return felt.Zero, err
	}
	l1HandlerTxnHash.Unmarshal(data)
	return l1HandlerTxnHash, nil
}

func DeleteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte) error {
	return w.Delete(db.L1HandlerTxnHashByMsgHashKey(msgHash))
}

func GetChainHeight(r db.KeyValueReader) (uint64, error) {
	data, err := r.Get(db.ChainHeight.Key())
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data), nil
}

func GetBlockHeaderByNumber(r db.KeyValueReader, number uint64) (*core.Header, error) {
	var header *core.Header
	data, err := r.Get(db.BlockHeaderByNumberKey(number))
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
	data, err := r.Get(db.BlockHeaderNumbersByHashKey(hash))
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(r, binary.BigEndian.Uint64(data))
}

func GetHeadsHeader(r db.KeyValueReader) (*core.Header, error) {
	height, err := GetChainHeight(r)
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(r, height)
}
