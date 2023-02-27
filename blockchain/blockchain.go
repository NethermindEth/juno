package blockchain

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

const lenOfByteSlice = 8

type ErrIncompatibleBlockAndStateUpdate struct {
	Err error
}

func (e ErrIncompatibleBlockAndStateUpdate) Error() string {
	return fmt.Sprintf("incompatible block and state update: %v", e.Err)
}

func (e ErrIncompatibleBlockAndStateUpdate) Unwrap() error {
	return e.Err
}

type ErrIncompatibleBlock struct {
	Err error
}

func (e ErrIncompatibleBlock) Error() string {
	return fmt.Sprintf("incompatible block: %v", e.Err.Error())
}

func (e ErrIncompatibleBlock) Unwrap() error {
	return e.Err
}

// Blockchain is responsible for keeping track of all things related to the Starknet blockchain
type Blockchain struct {
	network  utils.Network
	database db.DB
}

func New(database db.DB, network utils.Network) *Blockchain {
	return &Blockchain{
		database: database,
		network:  network,
	}
}

func (b *Blockchain) Network() utils.Network {
	return b.network
}

// StateCommitment returns the latest block state commitment.
// If blockchain is empty zero felt is returned.
func (b *Blockchain) StateCommitment() (commitment *felt.Felt, err error) {
	return commitment, b.database.View(func(txn db.Transaction) error {
		commitment, err = core.NewState(txn).Root()
		return err
	})
}

// Height returns the latest block height. If blockchain is empty nil is returned.
func (b *Blockchain) Height() (height uint64, err error) {
	return height, b.database.View(func(txn db.Transaction) error {
		height, err = b.height(txn)
		return err
	})
}

func (b *Blockchain) height(txn db.Transaction) (height uint64, err error) {
	return height, txn.Get(db.ChainHeight.Key(), func(val []byte) error {
		height = binary.BigEndian.Uint64(val)
		return nil
	})
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

func (b *Blockchain) GetStateUpdateByNumber(number uint64) (update *core.StateUpdate, err error) {
	return update, b.database.View(func(txn db.Transaction) error {
		update, err = getStateUpdateByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) GetStateUpdateByHash(hash *felt.Felt) (update *core.StateUpdate, err error) {
	return update, b.database.View(func(txn db.Transaction) error {
		update, err = getStateUpdateByHash(txn, hash)
		return err
	})
}

// GetTransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func (b *Blockchain) GetTransactionByBlockNumberAndIndex(blockNumber, index uint64) (transaction core.Transaction, err error) {
	return transaction, b.database.View(func(txn db.Transaction) error {
		transaction, err = getTransactionByBlockNumberAndIndex(txn, &txAndReceiptDBKey{blockNumber, index})
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

// GetReceipt gets the transaction receipt for a given transaction hash.
func (b *Blockchain) GetReceipt(hash *felt.Felt) (receipt *core.TransactionReceipt, err error) {
	return receipt, b.database.View(func(txn db.Transaction) error {
		receipt, err = getReceiptByHash(txn, hash)
		return err
	})
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, stateUpdate *core.StateUpdate) error {
	return b.database.Update(func(txn db.Transaction) error {
		if err := b.verifyBlock(txn, block); err != nil {
			return err
		}
		if err := core.NewState(txn).Update(stateUpdate); err != nil {
			return err
		}
		if err := storeBlockHeader(txn, &block.Header); err != nil {
			return err
		}
		for i, tx := range block.Transactions {
			if err := storeTransactionAndReceipt(txn, block.Number, uint64(i), tx,
				block.Receipts[i]); err != nil {
				return err
			}
		}

		if err := storeStateUpdate(txn, block.Number, stateUpdate); err != nil {
			return err
		}

		// Head of the blockchain is maintained as follows:
		// [db.ChainHeight]() -> (BlockNumber)
		heightBin := make([]byte, lenOfByteSlice)
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
	head, err := b.head(txn)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	if head == nil {
		if block.Number != 0 {
			return ErrIncompatibleBlock{
				errors.New("cannot insert a block with number more than 0 in an empty blockchain"),
			}
		}
		if !block.ParentHash.Equal(new(felt.Felt).SetUint64(0)) {
			return ErrIncompatibleBlock{errors.New(
				"cannot insert a block with non-zero parent hash in an empty blockchain")}
		}
	} else {
		if head.Number+1 != block.Number {
			return ErrIncompatibleBlock{
				errors.New("block number difference between head and incoming block is not 1"),
			}
		}
		if !block.ParentHash.Equal(head.Hash) {
			return ErrIncompatibleBlock{
				errors.New("block's parent hash does not match head block hash"),
			}
		}
	}

	return nil
}

// storeBlockHeader stores the given block in the database.
// The db storage for blocks is maintained by two buckets as follows:
//
// [db.BlockHeaderNumbersByHash](BlockHash) -> (BlockNumber)
// [db.BlockHeadersByNumber](BlockNumber) -> (Block)
//
// "[]" is the db prefix to represent a bucket
// "()" are additional keys appended to the prefix or multiple values marshalled together
// "->" represents a key value pair.
func storeBlockHeader(txn db.Transaction, block *core.Header) error {
	numBytes := make([]byte, lenOfByteSlice)
	binary.BigEndian.PutUint64(numBytes, block.Number)

	if err := txn.Set(db.BlockHeaderNumbersByHash.Key(block.Hash.Marshal()), numBytes); err != nil {
		return err
	}

	if blockBytes, err := encoder.Marshal(block); err != nil {
		return err
	} else if err = txn.Set(db.BlockHeadersByNumber.Key(numBytes), blockBytes); err != nil {
		return err
	}

	return nil
}

// getBlockByNumber retrieves a block from database by its number
func getBlockByNumber(txn db.Transaction, number uint64) (block *core.Block, err error) {
	numBytes := make([]byte, lenOfByteSlice)
	binary.BigEndian.PutUint64(numBytes, number)

	block = new(core.Block)
	if err = txn.Get(db.BlockHeadersByNumber.Key(numBytes), func(val []byte) error {
		return encoder.Unmarshal(val, &block.Header)
	}); err != nil {
		return nil, err
	}

	iterator, err := txn.NewIterator()
	if err != nil {
		return nil, err
	}
	defer func() {
		// Prioritise closing error over other errors
		if closeErr := iterator.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	prefix := db.TransactionsByBlockNumberAndIndex.Key(numBytes)
	for iterator.Seek(prefix); iterator.Valid(); iterator.Next() {
		if !bytes.Equal(iterator.Key()[:len(prefix)], prefix) {
			break
		}

		val, err := iterator.Value()
		if err != nil {
			return nil, err
		}

		var tx core.Transaction
		if err = encoder.Unmarshal(val, &tx); err != nil {
			return nil, err
		}

		block.Transactions = append(block.Transactions, tx)
	}

	prefix = db.ReceiptsByBlockNumberAndIndex.Key(numBytes)
	for iterator.Seek(prefix); iterator.Valid(); iterator.Next() {
		if !bytes.Equal(iterator.Key()[:len(prefix)], prefix) {
			break
		}

		val, err := iterator.Value()
		if err != nil {
			return nil, err
		}

		receipt := new(core.TransactionReceipt)
		if err = encoder.Unmarshal(val, receipt); err != nil {
			return nil, err
		}

		block.Receipts = append(block.Receipts, receipt)
	}

	return block, nil
}

// getBlockByHash retrieves a block from database by its hash
func getBlockByHash(txn db.Transaction, hash *felt.Felt) (block *core.Block, err error) {
	return block, txn.Get(db.BlockHeaderNumbersByHash.Key(hash.Marshal()), func(val []byte) error {
		block, err = getBlockByNumber(txn, binary.BigEndian.Uint64(val))
		return err
	})
}

func storeStateUpdate(txn db.Transaction, blockNumber uint64, update *core.StateUpdate) error {
	numBytes := make([]byte, lenOfByteSlice)
	binary.BigEndian.PutUint64(numBytes, blockNumber)

	if updateBytes, err := encoder.Marshal(update); err != nil {
		return err
	} else if err = txn.Set(db.StateUpdatesByBlockNumber.Key(numBytes), updateBytes); err != nil {
		return err
	}

	return nil
}

func getStateUpdateByNumber(txn db.Transaction, blockNumber uint64) (update *core.StateUpdate, err error) {
	numBytes := make([]byte, lenOfByteSlice)
	binary.BigEndian.PutUint64(numBytes, blockNumber)

	return update, txn.Get(db.StateUpdatesByBlockNumber.Key(numBytes), func(val []byte) error {
		update = new(core.StateUpdate)
		return encoder.Unmarshal(val, update)
	})
}

func getStateUpdateByHash(txn db.Transaction, hash *felt.Felt) (update *core.StateUpdate, err error) {
	return update, txn.Get(db.BlockHeaderNumbersByHash.Key(hash.Marshal()), func(val []byte) error {
		update, err = getStateUpdateByNumber(txn, binary.BigEndian.Uint64(val))
		return err
	})
}

// SanityCheckNewHeight checks integrity of a block and resulting state update
func (b *Blockchain) SanityCheckNewHeight(block *core.Block, stateUpdate *core.StateUpdate) error {
	if !block.Hash.Equal(stateUpdate.BlockHash) {
		return ErrIncompatibleBlockAndStateUpdate{errors.New("block hashes do not match")}
	}
	if !block.GlobalStateRoot.Equal(stateUpdate.NewRoot) {
		return ErrIncompatibleBlockAndStateUpdate{errors.New("block's GlobalStateRoot does not match state update's NewRoot")}
	}
	return core.VerifyBlockHash(block, b.network)
}

type txAndReceiptDBKey struct {
	Number uint64
	Index  uint64
}

func (t *txAndReceiptDBKey) MarshalBinary() []byte {
	return binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64([]byte{}, t.Number), t.Index)
}

func (t *txAndReceiptDBKey) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)
	if err := binary.Read(r, binary.BigEndian, &t.Number); err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &t.Index)
}

// storeTransactionAndReceipt stores the given transaction receipt in the database.
// The db storage for transaction and receipts is maintained by three buckets as follows:
//
// [db.TransactionBlockNumbersAndIndicesByHash](TransactionHash) -> (BlockNumber, Index)
// [db.TransactionsByBlockNumberAndIndex](BlockNumber, Index) -> Transaction
// [db.ReceiptsByBlockNumberAndIndex](BlockNumber, Index) -> Receipt
//
// Note: we are using the same transaction hash bucket which keeps track of block number and
// index for both transactions and receipts since transaction and its receipt share the same hash.
// "[]" is the db prefix to represent a bucket
// "()" are additional keys appended to the prefix or multiple values marshalled together
// "->" represents a key value pair.
func storeTransactionAndReceipt(txn db.Transaction, number, i uint64, t core.Transaction, r *core.TransactionReceipt) error {
	bnIndexBytes := (&txAndReceiptDBKey{number, i}).MarshalBinary()

	if err := txn.Set(db.TransactionBlockNumbersAndIndicesByHash.Key((r.TransactionHash).Marshal()),
		bnIndexBytes); err != nil {
		return err
	}

	if txnBytes, err := encoder.Marshal(t); err != nil {
		return err
	} else if err = txn.Set(db.TransactionsByBlockNumberAndIndex.Key(bnIndexBytes), txnBytes); err != nil {
		return err
	}

	if rBytes, err := encoder.Marshal(r); err != nil {
		return err
	} else if err = txn.Set(db.ReceiptsByBlockNumberAndIndex.Key(bnIndexBytes), rBytes); err != nil {
		return err
	}
	return nil
}

// getTransactionBlockNumberAndIndexByHash gets the block number and index for a given transaction hash
func getTransactionBlockNumberAndIndexByHash(txn db.Transaction, hash *felt.Felt) (bnIndex *txAndReceiptDBKey, err error) {
	return bnIndex, txn.Get(db.TransactionBlockNumbersAndIndicesByHash.Key(hash.Marshal()), func(val []byte) error {
		bnIndex = new(txAndReceiptDBKey)
		return bnIndex.UnmarshalBinary(val)
	})
}

// getTransactionByBlockNumberAndIndex gets the transaction for a given block number and index.
func getTransactionByBlockNumberAndIndex(txn db.Transaction, bnIndex *txAndReceiptDBKey) (transaction core.Transaction, err error) {
	return transaction, txn.Get(db.TransactionsByBlockNumberAndIndex.Key(bnIndex.MarshalBinary()), func(val []byte) error {
		return encoder.Unmarshal(val, &transaction)
	})
}

// getTransactionByHash gets the transaction for a given hash.
func getTransactionByHash(txn db.Transaction, hash *felt.Felt) (core.Transaction, error) {
	bnIndex, err := getTransactionBlockNumberAndIndexByHash(txn, hash)
	if err != nil {
		return nil, err
	}
	return getTransactionByBlockNumberAndIndex(txn, bnIndex)
}

// getReceiptByHash gets the transaction receipt for a given hash.
func getReceiptByHash(txn db.Transaction, hash *felt.Felt) (*core.TransactionReceipt, error) {
	if bnIndex, err := getTransactionBlockNumberAndIndexByHash(txn, hash); err != nil {
		return nil, err
	} else {
		return getReceiptByBlockNumberAndIndex(txn, bnIndex)
	}
}

// getReceiptByBlockNumberAndIndex gets the transaction receipt for a given block number and index.
func getReceiptByBlockNumberAndIndex(txn db.Transaction, bnIndex *txAndReceiptDBKey) (r *core.TransactionReceipt, err error) {
	return r, txn.Get(db.ReceiptsByBlockNumberAndIndex.Key(bnIndex.MarshalBinary()), func(val []byte) error {
		return encoder.Unmarshal(val, &r)
	})
}
