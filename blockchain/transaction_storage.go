package blockchain

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// TransactionStorage is a storage that allows us to store and retrieve the block number and index from the database
type TransactionStorage struct {
	txn db.Transaction
}

// NewTransactionStorage creates a new TransactionStorage
func NewTransactionStorage(txn db.Transaction) *TransactionStorage {
	return &TransactionStorage{txn: txn}
}

// Put stores the block number and index for a given transaction hash
func (s *TransactionStorage) Put(blockNumber, index uint64, hash *felt.Felt) error {
	var bNumberBytes [8]byte
	binary.BigEndian.PutUint64(bNumberBytes[:], blockNumber)
	var indexBytes [8]byte
	binary.BigEndian.PutUint64(indexBytes[:], index)
	value := append(bNumberBytes[:], indexBytes[:]...)
	if err := s.txn.Set(db.TransactionsIndexByHash.Key(hash.Marshal()), value); err != nil {
		return err
	}

	return nil
}

// GetBlockNumberAndIndex gets the block number and index for a given transaction hash
func (s *TransactionStorage) GetBlockNumberAndIndex(hash *felt.Felt) (*uint64, *uint64, error) {
	value, err := s.txn.Get(db.TransactionsIndexByHash.Key(hash.Marshal()))
	if err != nil {
		return nil, nil, err
	}

	blockNumber := binary.BigEndian.Uint64(value[:8])
	index := binary.BigEndian.Uint64(value[8:])
	return &blockNumber, &index, nil
}
