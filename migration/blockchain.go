package migration

import (
	"bytes"
	"encoding/binary"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

func blockByNumber(txn db.Transaction, number uint64) (*core.Block, error) {
	header, err := blockHeaderByNumber(txn, number)
	if err != nil {
		return nil, err
	}

	block := new(core.Block)
	block.Header = header
	block.Transactions, err = transactionsByBlockNumber(txn, number)
	if err != nil {
		return nil, err
	}

	block.Receipts, err = receiptsByBlockNumber(txn, number)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func blockHeaderByNumber(txn db.Transaction, number uint64) (*core.Header, error) {
	numBytes := core.MarshalBlockNumber(number)

	var header *core.Header
	if err := txn.Get(db.BlockHeadersByNumber.Key(numBytes), func(val []byte) error {
		header = new(core.Header)
		return encoder.Unmarshal(val, header)
	}); err != nil {
		return nil, err
	}
	return header, nil
}

func head(txn db.Transaction) (*core.Block, error) {
	height, err := chainHeight(txn)
	if err != nil {
		return nil, err
	}
	return blockByNumber(txn, height)
}

func chainHeight(txn db.Transaction) (uint64, error) {
	var height uint64
	return height, txn.Get(db.ChainHeight.Key(), func(val []byte) error {
		height = binary.BigEndian.Uint64(val)
		return nil
	})
}

func transactionsByBlockNumber(txn db.Transaction, number uint64) ([]core.Transaction, error) {
	numBytes := core.MarshalBlockNumber(number)
	prefix := db.TransactionsByBlockNumberAndIndex.Key(numBytes)

	iterator, err := txn.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}

	var txs []core.Transaction
	for iterator.First(); iterator.Valid(); iterator.Next() {
		val, vErr := iterator.Value()
		if vErr != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, vErr)
		}

		var tx core.Transaction
		if err = encoder.Unmarshal(val, &tx); err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		}

		txs = append(txs, tx)
	}

	if err = iterator.Close(); err != nil {
		return nil, err
	}

	return txs, nil
}

func receiptsByBlockNumber(txn db.Transaction, number uint64) ([]*core.TransactionReceipt, error) {
	numBytes := core.MarshalBlockNumber(number)
	prefix := db.ReceiptsByBlockNumberAndIndex.Key(numBytes)

	iterator, err := txn.NewIterator(prefix, true)
	if err != nil {
		return nil, err
	}

	var receipts []*core.TransactionReceipt

	for iterator.First(); iterator.Valid(); iterator.Next() {
		if !bytes.HasPrefix(iterator.Key(), prefix) {
			break
		}

		val, vErr := iterator.Value()
		if vErr != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, vErr)
		}

		receipt := new(core.TransactionReceipt)
		if err = encoder.Unmarshal(val, receipt); err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		}

		receipts = append(receipts, receipt)
	}

	if err = iterator.Close(); err != nil {
		return nil, err
	}

	return receipts, nil
}

func storeBlockHeader(txn db.Transaction, header *core.Header) error {
	numBytes := core.MarshalBlockNumber(header.Number)

	if err := txn.Set(db.BlockHeaderNumbersByHash.Key(header.Hash.Marshal()), numBytes); err != nil {
		return err
	}

	headerBytes, err := encoder.Marshal(header)
	if err != nil {
		return err
	}

	return txn.Set(db.BlockHeadersByNumber.Key(numBytes), headerBytes)
}

func storeBlockCommitments(txn db.Transaction, blockNumber uint64, commitments *core.BlockCommitments) error {
	numBytes := core.MarshalBlockNumber(blockNumber)

	commitmentBytes, err := encoder.Marshal(commitments)
	if err != nil {
		return err
	}

	return txn.Set(db.BlockCommitments.Key(numBytes), commitmentBytes)
}

func storeL1HandlerMsgHashes(dbTxn db.Transaction, blockTxns []core.Transaction) error {
	for _, txn := range blockTxns {
		if l1Handler, ok := (txn).(*core.L1HandlerTransaction); ok {
			err := dbTxn.Set(db.L1HandlerTxnHashByMsgHash.Key(l1Handler.MessageHash()), txn.Hash().Marshal())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
