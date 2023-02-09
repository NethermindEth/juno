package blockchain

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

const lenOfBlockNumberBytes = 8

type ErrIncompatibleBlockAndStateUpdate struct {
	reason string
}

func (e ErrIncompatibleBlockAndStateUpdate) Error() string {
	return fmt.Sprintf("incompatible block and state update: %v", e.reason)
}

type ErrIncompatibleBlock struct {
	reason string
}

func (e ErrIncompatibleBlock) Error() string {
	return fmt.Sprintf("incompatible block: %v", e.reason)
}

type ErrIncompatibleTransaction struct {
	reason string
}

func (e ErrIncompatibleTransaction) Error() string {
	return fmt.Sprintf("incompatible transaction: %v", e.reason)
}

// Blockchain is responsible for keeping track of all things related to the StarkNet blockchain
type Blockchain struct {
	network  utils.Network
	database db.DB
}

func NewBlockchain(database db.DB, network utils.Network) *Blockchain {
	return &Blockchain{
		database: database,
		network:  network,
	}
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain) Height() (height uint64, err error) {
	return height, b.database.View(func(txn db.Transaction) error {
		height, err = b.height(txn)
		return err
	})
}

func (b *Blockchain) height(txn db.Transaction) (uint64, error) {
	heightBin, err := txn.Get(db.ChainHeight.Key())
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(heightBin), err
}

func (b *Blockchain) Head() (head *core.Block, err error) {
	return head, b.database.View(func(txn db.Transaction) error {
		head, err = b.head(txn)
		return err
	})
}

func (b *Blockchain) head(txn db.Transaction) (*core.Block, error) {
	if height, err := b.height(txn); err != nil {
		return nil, err
	} else {
		return getBlockByNumber(txn, height)
	}
}

func (b *Blockchain) GetBlockByNumber(number uint64) (block *core.Block, err error) {
	return block, b.database.View(func(txn db.Transaction) error {
		block, err = getBlockByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) GetBlockByHash(hash *felt.Felt) (block *core.Block, err error) {
	return block, b.database.View(func(txn db.Transaction) error {
		block, err = getBlockByHash(txn, hash)
		return err
	})
}

// StoreBlock takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) StoreBlock(block *core.Block, stateUpdate *core.StateUpdate) error {
	return b.database.Update(func(txn db.Transaction) error {
		if err := b.verifyBlock(txn, block); err != nil {
			return err
		}
		if err := state.NewState(txn).Update(stateUpdate); err != nil {
			return err
		}
		if err := putBlock(txn, block); err != nil {
			return err
		}

		heightBin := make([]byte, lenOfBlockNumberBytes)
		binary.BigEndian.PutUint64(heightBin, block.Number)
		return txn.Set(db.ChainHeight.Key(), heightBin)
	})
}

// VerifyBlock assumes the block has already been sanity-checked.
func (b *Blockchain) VerifyBlock(block *core.Block) error {
	return b.database.View(func(txn db.Transaction) error {
		return b.verifyBlock(txn, block)
	})
}

func (b *Blockchain) verifyBlock(txn db.Transaction, block *core.Block) error {
	/*
		Todo: Transaction and TransactionReceipts
			- When Block is changed to include a list of Transaction and TransactionReceipts
			- Further checks would need to be added to ensure Transaction Hash has been computed
				properly.
			- Sanity check would need to include checks which ensure there is same number of
				Transactions and TransactionReceipts.
	*/
	head, err := b.head(txn)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	if head == nil {
		if block.Number != 0 {
			return &ErrIncompatibleBlock{
				"cannot insert a block with number more than 0 in an empty blockchain",
			}
		}
		if !block.ParentHash.Equal(new(felt.Felt).SetUint64(0)) {
			return &ErrIncompatibleBlock{
				"cannot insert a block with non-zero parent hash in an empty blockchain",
			}
		}
	} else {
		if head.Number+1 != block.Number {
			return &ErrIncompatibleBlock{
				"block number difference between head and incoming block is not 1",
			}
		}
		if !block.ParentHash.Equal(head.Hash) {
			return &ErrIncompatibleBlock{
				"block's parent hash does not match head block hash",
			}
		}
	}

	return nil
}

// putBlock stores the given block in the database. No check on whether the hash matches or not is done
func putBlock(txn db.Transaction, block *core.Block) error {
	numBytes := make([]byte, lenOfBlockNumberBytes)
	binary.BigEndian.PutUint64(numBytes, block.Number)
	if err := txn.Set(db.BlockNumbersByHash.Key(block.Hash.Marshal()), numBytes); err != nil {
		return err
	}

	if blockBytes, err := encoder.Marshal(block); err != nil {
		return err
	} else if err = txn.Set(db.BlocksByNumber.Key(numBytes), blockBytes); err != nil {
		return err
	}

	return nil
}

// getBlockByNumber retrieves a block from database by its number
func getBlockByNumber(txn db.Transaction, number uint64) (*core.Block, error) {
	numBytes := make([]byte, lenOfBlockNumberBytes)
	binary.BigEndian.PutUint64(numBytes, number)
	return getBlockByNumberBytes(txn, numBytes)
}

func getBlockByNumberBytes(txn db.Transaction, numBytes []byte) (*core.Block, error) {
	if blockBytes, err := txn.Get(db.BlocksByNumber.Key(numBytes)); err != nil {
		return nil, err
	} else {
		block := new(core.Block)
		return block, encoder.Unmarshal(blockBytes, block)
	}
}

// getBlockByHash retrieves a block from database by its hash
func getBlockByHash(txn db.Transaction, hash *felt.Felt) (*core.Block, error) {
	if numBytes, err := txn.Get(db.BlockNumbersByHash.Key(hash.Marshal())); err != nil {
		return nil, err
	} else {
		return getBlockByNumberBytes(txn, numBytes)
	}
}

// SanityCheckNewHeight checks integrity of a block and resulting state update
func (b *Blockchain) SanityCheckNewHeight(block *core.Block, stateUpdate *core.StateUpdate) error {
	if !block.Hash.Equal(stateUpdate.BlockHash) {
		return ErrIncompatibleBlockAndStateUpdate{"block hashes do not match"}
	}
	if !block.GlobalStateRoot.Equal(stateUpdate.NewRoot) {
		return ErrIncompatibleBlockAndStateUpdate{
			"block's GlobalStateRoot does not match state update's NewRoot",
		}
	}

	return core.VerifyBlockHash(block, b.network)
}

// GetTransactionByBlockNumAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) GetTransactionByBlockNumAndIndex(blockNumber, index uint64) (transaction core.Transaction, err error) {
	bnIndexBytes := joinBlockNumberAndIndex(blockNumber, index)
	return transaction, b.database.View(func(txn db.Transaction) error {
		transaction, err = getTransactionByBnIndexBytes(txn, bnIndexBytes)
		return err
	})
}

// GetTransactionByHash gets the transaction for a given hash.
func (b *Blockchain) GetTransactionByHash(hash *felt.Felt) (transaction core.Transaction, err error) {
	return transaction, b.database.View(func(txn db.Transaction) error {
		transaction, err = getTransactionByHash(txn, hash)
		return err
	})
}

// StoreTransaction takes a transaction and verifies its hash before storing it in the database.
func (b *Blockchain) StoreTransaction(blockNumber, txnIndex uint64, transaction core.Transaction) error {
	return b.database.Update(func(txn db.Transaction) error {
		if hash, err := b.VerifyTransaction(transaction); err != nil {
			return err
		} else {
			bnIndexBytes := joinBlockNumberAndIndex(blockNumber, txnIndex)
			return putTransaction(txn, bnIndexBytes, hash, transaction)
		}
	})
}

func (b *Blockchain) VerifyTransaction(transaction core.Transaction) (*felt.Felt, error) {
	txn := b.database.NewTransaction(false)
	defer txn.Discard()
	return b.verifyTransaction(txn, transaction)
}

func (b *Blockchain) verifyTransaction(txn db.Transaction, transaction core.Transaction) (*felt.Felt, error) {
	txnHash, err := core.TransactionHash(transaction, b.network)
	if err != nil {
		return nil, err
	}
	if !hash.Equal(txnHash) {
		return txnHash, &ErrIncompatibleTransaction{fmt.Sprintf(
			"incorrect transaction hash: hash = %v and transaction.Hash(b.network) = %v",
			hash.Text(16), txnHash.Text(16))}, nil
	}

	return txnHash, nil
}

// putTransaction stores the given transaction in the database.
func putTransaction(txn db.Transaction, bnIndexBytes []byte, hash *felt.Felt, transaction core.Transaction) error {
	if err := txn.Set(db.TransactionsIndexByHash.Key(hash.Marshal()), bnIndexBytes); err != nil {
		return err
	}

	if txnBytes, err := encoder.Marshal(transaction); err != nil {
		return err
	} else if err = txn.Set(db.Transactions.Key(bnIndexBytes), txnBytes); err != nil {
		return err
	}
	return nil
}

// getBlockNumberAndIndex gets the block number and index for a given transaction hash
func getBlockNumberAndIndex(txn db.Transaction, hash *felt.Felt) ([]byte, error) {
	bnIndexBytes, err := txn.Get(db.TransactionsIndexByHash.Key(hash.Marshal()))
	if err != nil {
		return nil, err
	}

	return bnIndexBytes, nil
}

// getTransactionByBnIndexBytes gets the transaction for a given block number and index.
func getTransactionByBnIndexBytes(txn db.Transaction, bnIndexBytes []byte) (core.Transaction, error) {
	if value, err := txn.Get(db.Transactions.Key(bnIndexBytes)); err != nil {
		return nil, err
	} else {
		var transaction core.Transaction
		if err := encoder.Unmarshal(value, &transaction); err != nil {
			return nil, err
		}
		return transaction, nil
	}
}

// getTransactionByHash gets the transaction for a given hash.
func getTransactionByHash(txn db.Transaction, hash *felt.Felt) (core.Transaction, error) {
	bnIndexBytes, err := getBlockNumberAndIndex(txn, hash)
	if err != nil {
		return nil, err
	}
	return getTransactionByBnIndexBytes(txn, bnIndexBytes)
}

// GetReceipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain) GetReceipt(hash *felt.Felt) (receipt *core.TransactionReceipt, err error) {
	return receipt, b.database.View(func(txn db.Transaction) error {
		receipt, err = getReceiptByHash(txn, hash)
		return err
	})
}

// StoreReceipt takes a transaction receipt and stores it in the database.
func (b *Blockchain) StoreReceipt(blockNumber uint64, receipt core.TransactionReceipt) error {
	return b.database.Update(func(txn db.Transaction) error {
		bnIndexBytes := joinBlockNumberAndIndex(blockNumber, receipt.TransactionIndex)
		return putReceipt(txn, bnIndexBytes, &receipt)
	})
}

// putReceipt stores the given transaction receipt in the database.
func putReceipt(txn db.Transaction, bnIndexBytes []byte, receipt *core.TransactionReceipt) error {
	if err := txn.Set(db.TransactionsIndexByHash.Key((receipt.TransactionHash).Marshal()), bnIndexBytes); err != nil {
		return err
	}

	if txnBytes, err := encoder.Marshal(receipt); err != nil {
		return err
	} else {
		return txn.Set(db.TransactionReceipts.Key(bnIndexBytes), txnBytes)
	}
}

// getReceipt gets the transaction receipt for a given block number and index.
func getReceipt(txn db.Transaction, bnIndexBytes []byte) (*core.TransactionReceipt, error) {
	value, err := txn.Get(db.TransactionReceipts.Key(bnIndexBytes))
	if err != nil {
		return nil, err
	}

	var receipt *core.TransactionReceipt
	if err := encoder.Unmarshal(value, &receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}

// getReceiptByHash gets the transaction receipt for a given hash.
func getReceiptByHash(txn db.Transaction, hash *felt.Felt) (*core.TransactionReceipt, error) {
	if bnIndexBytes, err := getBlockNumberAndIndex(txn, hash); err != nil {
		return nil, err
	} else {
		return getReceipt(txn, bnIndexBytes)
	}
}

func joinBlockNumberAndIndex(blockNumber, index uint64) []byte {
	var bNumberBytes [8]byte
	binary.BigEndian.PutUint64(bNumberBytes[:], blockNumber)
	var indexBytes [8]byte
	binary.BigEndian.PutUint64(indexBytes[:], index)
	return append(bNumberBytes[:], indexBytes[:]...)
}
