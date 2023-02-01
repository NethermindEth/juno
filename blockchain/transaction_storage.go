package blockchain

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/fxamacker/cbor/v2"
)

// TransactionStorage is a storage that allows us to store and retrieve the block number and index from the database
type TransactionStorage struct {
	txn db.Transaction
}

// NewTransactionStorage creates a new TransactionStorage
func NewTransactionStorage(txn db.Transaction) *TransactionStorage {
	return &TransactionStorage{txn: txn}
}

// PutTransaction stores the given transaction in the database.
// TODO: Remove hash from the arguments once it has been added to the transaction.
func (s *TransactionStorage) PutTransaction(blockNumber, index uint64, hash *felt.Felt, transaction *core.Transaction) error {
	value := joinBlockNumberAndIndex(blockNumber, index)
	if err := s.txn.Set(db.TransactionsIndexByHash.Key(hash.Marshal()), value); err != nil {
		return err
	}
	fmt.Println(value)

	if txnBytes, err := cbor.Marshal(transaction); err != nil {
		return err
	} else {
		return s.txn.Set(db.Transactions.Key(value), txnBytes)
	}
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

// GetTransaction gets the transaction for a given block number and index.
func (s *TransactionStorage) GetTransaction(blockNumber, index uint64) (*core.Transaction, error) {
	key := joinBlockNumberAndIndex(blockNumber, index)
	fmt.Println(key)
	value, err := s.txn.Get(db.Transactions.Key(key))
	if err != nil {
		return nil, err
	}

	var transaction *core.Transaction
	if err := cbor.Unmarshal(value, &transaction); err != nil {
		return nil, err
	}
	return transaction, nil
}

// PutTransactionReceipt stores the given transaction receipt in the database.
func (s *TransactionStorage) PutTransactionReceipt(blockNumber, index uint64, receipt *core.TransactionReceipt) error {
	key := joinBlockNumberAndIndex(blockNumber, index)
	if txnBytes, err := cbor.Marshal(receipt); err != nil {
		return err
	} else {
		return s.txn.Set(db.TransactionReceipts.Key(key), txnBytes)
	}
}

// GetTransactionReceipt gets the transaction receipt for a given block number and index.
func (s *TransactionStorage) GetTransactionReceipt(blockNumber, index uint64) (*core.TransactionReceipt, error) {
	key := joinBlockNumberAndIndex(blockNumber, index)
	value, err := s.txn.Get(db.TransactionReceipts.Key(key))
	if err != nil {
		return nil, err
	}

	var receipt *core.TransactionReceipt
	if err := cbor.Unmarshal(value, &receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

func joinBlockNumberAndIndex(blockNumber, index uint64) []byte {
	var bNumberBytes [8]byte
	binary.BigEndian.PutUint64(bNumberBytes[:], blockNumber)
	var indexBytes [8]byte
	binary.BigEndian.PutUint64(indexBytes[:], index)
	return append(bNumberBytes[:], indexBytes[:]...)
}
