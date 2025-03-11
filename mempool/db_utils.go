package mempool

import (
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

func headValue(txn db.Transaction, head *felt.Felt) error {
	return txn.Get(db.MempoolHead.Key(), func(b []byte) error {
		head.SetBytes(b)
		return nil
	})
}

func tailValue(txn db.Transaction, tail *felt.Felt) error {
	return txn.Get(db.MempoolTail.Key(), func(b []byte) error {
		tail.SetBytes(b)
		return nil
	})
}

func updateHead(txn db.Transaction, head *felt.Felt) error {
	return txn.Set(db.MempoolHead.Key(), head.Marshal())
}

func updateTail(txn db.Transaction, tail *felt.Felt) error {
	return txn.Set(db.MempoolTail.Key(), tail.Marshal())
}

func readTxn(txn db.Transaction, itemKey *felt.Felt) (dbPoolTxn, error) {
	var item dbPoolTxn
	err := txn.Get(db.MempoolNodeKey(itemKey), func(b []byte) error {
		return encoder.Unmarshal(b, &item)
	})
	return item, err
}

func setTxn(txn db.Transaction, item *dbPoolTxn) error {
	itemBytes, err := encoder.Marshal(item)
	if err != nil {
		return err
	}
	return txn.Set(db.MempoolNodeKey(item.Txn.Transaction.Hash()), itemBytes)
}

func lenDB(txn db.Transaction) (int, error) {
	var l int
	err := txn.Get(db.MempoolLength.Key(), func(b []byte) error {
		l = int(new(big.Int).SetBytes(b).Int64())
		return nil
	})

	if err != nil && errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	return l, err
}

func GetHeadValue(r db.KeyValueReader) (felt.Felt, error) {
	var head felt.Felt
	data, err := r.Get2(db.MempoolHead.Key())
	if err != nil {
		return felt.Zero, err
	}
	head.Unmarshal(data)
	return head, nil
}

func GetTailValue(r db.KeyValueReader) (felt.Felt, error) {
	var tail felt.Felt
	data, err := r.Get2(db.MempoolTail.Key())
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
	data, err := r.Get2(db.MempoolNodeKey(txnHash))
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
	data, err := r.Get2(db.MempoolLength.Key())
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
