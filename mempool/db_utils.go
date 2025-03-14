package mempool

import (
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

func GetHeadValue(r db.KeyValueReader) (felt.Felt, error) {
	var head felt.Felt
	data, err := r.Get(db.MempoolHead.Key())
	if err != nil {
		return felt.Zero, err
	}
	head.Unmarshal(data)
	return head, nil
}

func GetTailValue(r db.KeyValueReader) (felt.Felt, error) {
	var tail felt.Felt
	data, err := r.Get(db.MempoolTail.Key())
	if err != nil {
		return felt.Zero, err
	}
	tail.Unmarshal(data)
	return tail, nil
}

func WriteHeadValue(w db.KeyValueWriter, head *felt.Felt) error {
	return w.Put(db.MempoolHead.Key(), head.Marshal())
}

func WriteTailValue(w db.KeyValueWriter, tail *felt.Felt) error {
	return w.Put(db.MempoolTail.Key(), tail.Marshal())
}

func GetTxn(r db.KeyValueReader, txnHash *felt.Felt) (dbPoolTxn, error) {
	var item dbPoolTxn
	data, err := r.Get(db.MempoolNodeKey(txnHash))
	if err != nil {
		return dbPoolTxn{}, err
	}
	err = encoder.Unmarshal(data, &item)
	if err != nil {
		return dbPoolTxn{}, err
	}
	return item, nil
}

func WriteTxn(w db.KeyValueWriter, item *dbPoolTxn) error {
	itemBytes, err := encoder.Marshal(item)
	if err != nil {
		return err
	}
	return w.Put(db.MempoolNodeKey(item.Txn.Transaction.Hash()), itemBytes)
}

func GetLenDB(r db.KeyValueReader) (int, error) {
	var l int
	data, err := r.Get(db.MempoolLength.Key())
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	l = int(new(big.Int).SetBytes(data).Int64())
	return l, nil
}

func WriteLenDB(w db.KeyValueWriter, l int) error {
	return w.Put(db.MempoolLength.Key(), big.NewInt(int64(l)).Bytes())
}
