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
	keyBytes := itemKey.Bytes()
	err := txn.Get(db.MempoolNode.Key(keyBytes[:]), func(b []byte) error {
		return encoder.Unmarshal(b, &item)
	})
	return item, err
}

func setTxn(txn db.Transaction, item *dbPoolTxn) error {
	itemBytes, err := encoder.Marshal(item)
	if err != nil {
		return err
	}
	keyBytes := item.Txn.Transaction.Hash().Bytes()
	return txn.Set(db.MempoolNode.Key(keyBytes[:]), itemBytes)
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
