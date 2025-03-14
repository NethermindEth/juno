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
	err := r.Get(db.MempoolHead.Key(), func(data []byte) error {
		head.Unmarshal(data)
		return nil
	})
	return head, err
}

func GetTailValue(r db.KeyValueReader) (felt.Felt, error) {
	var tail felt.Felt
	err := r.Get(db.MempoolTail.Key(), func(data []byte) error {
		tail.Unmarshal(data)
		return nil
	})
	return tail, err
}

func WriteHeadValue(w db.KeyValueWriter, head *felt.Felt) error {
	return w.Put(db.MempoolHead.Key(), head.Marshal())
}

func WriteTailValue(w db.KeyValueWriter, tail *felt.Felt) error {
	return w.Put(db.MempoolTail.Key(), tail.Marshal())
}

func GetTxn(r db.KeyValueReader, txnHash *felt.Felt) (dbPoolTxn, error) {
	var item dbPoolTxn
	err := r.Get(db.MempoolNodeKey(txnHash), func(data []byte) error {
		return encoder.Unmarshal(data, &item)
	})
	return item, err
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
	err := r.Get(db.MempoolLength.Key(), func(data []byte) error {
		l = int(new(big.Int).SetBytes(data).Int64())
		return nil
	})
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return l, nil
}

func WriteLenDB(w db.KeyValueWriter, l int) error {
	return w.Put(db.MempoolLength.Key(), big.NewInt(int64(l)).Bytes())
}
