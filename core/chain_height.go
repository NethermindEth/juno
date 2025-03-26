package core

import (
	"github.com/NethermindEth/juno/db"
)

// Head of the blockchain is maintained as follows:
// [db.ChainHeight]() -> (BlockNumber)
func ChainHeight(txn db.Transaction) (uint64, error) {
	var height uint64
	return height, txn.Get(db.ChainHeight.Key(), func(val []byte) error {
		height = UnmarshalBlockNumber(val)
		return nil
	})
}

func SetChainHeight(txn db.Transaction, height uint64) error {
	return txn.Set(db.ChainHeight.Key(), MarshalBlockNumber(height))
}

func DeleteChainHeight(txn db.Transaction) error {
	return txn.Delete(db.ChainHeight.Key())
}
