package blockchain

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/fxamacker/cbor/v2"
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
		return getByNumber(txn, height)
	}
}

func (b *Blockchain) GetBlockByNumber(number uint64) (block *core.Block, err error) {
	return block, b.database.View(func(txn db.Transaction) error {
		block, err = getByNumber(txn, number)
		return err
	})
}

func (b *Blockchain) GetBlockByHash(hash *felt.Felt) (block *core.Block, err error) {
	return block, b.database.View(func(txn db.Transaction) error {
		block, err = getByHash(txn, hash)
		return err
	})
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, stateUpdate *core.StateUpdate) error {
	return b.database.Update(func(txn db.Transaction) error {
		if err := b.verifyBlock(txn, block, stateUpdate); err != nil {
			return err
		}
		if err := state.NewState(txn).Update(stateUpdate); err != nil {
			return err
		}
		if err := put(txn, block); err != nil {
			return err
		}

		heightBin := make([]byte, lenOfBlockNumberBytes)
		binary.BigEndian.PutUint64(heightBin, block.Number)
		return txn.Set(db.ChainHeight.Key(), heightBin)
	})
}

func (b *Blockchain) VerifyBlock(block *core.Block, stateUpdate *core.StateUpdate) error {
	txn := b.database.NewTransaction(false)
	defer txn.Discard()
	return b.verifyBlock(txn, block, stateUpdate)
}

func (b *Blockchain) verifyBlock(txn db.Transaction, block *core.Block,
	stateUpdate *core.StateUpdate,
) error {
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

	if !block.Hash.Equal(stateUpdate.BlockHash) {
		return ErrIncompatibleBlockAndStateUpdate{"block hashes do not match"}
	}
	if !block.GlobalStateRoot.Equal(stateUpdate.NewRoot) {
		return ErrIncompatibleBlockAndStateUpdate{
			"block's GlobalStateRoot does not match state update's NewRoot",
		}
	}

	h, err := core.BlockHash(block, b.network)
	if err != nil && !errors.As(err, new(*core.ErrUnverifiableBlock)) {
		return err
	}

	if h != nil && !block.Hash.Equal(h) {
		return &ErrIncompatibleBlock{fmt.Sprintf(
			"incorrect block hash: block.Hash = %v and BlockHash(block) = %v",
			block.Hash.Text(16), h.Text(16))}
	}

	return nil
}

// Put stores the given block in the database. No check on whether the hash matches or not is done
func put(txn db.Transaction, block *core.Block) error {
	numBytes := make([]byte, lenOfBlockNumberBytes)
	binary.BigEndian.PutUint64(numBytes, block.Number)
	if err := txn.Set(db.BlockNumbersByHash.Key(block.Hash.Marshal()), numBytes); err != nil {
		return err
	}

	if blockBytes, err := cbor.Marshal(block); err != nil {
		return err
	} else if err = txn.Set(db.BlocksByNumber.Key(numBytes), blockBytes); err != nil {
		return err
	}

	return nil
}

// getByNumber retrieves a block from database by its number
func getByNumber(txn db.Transaction, number uint64) (*core.Block, error) {
	numBytes := make([]byte, lenOfBlockNumberBytes)
	binary.BigEndian.PutUint64(numBytes, number)
	return getByNumberBytes(txn, numBytes)
}

func getByNumberBytes(txn db.Transaction, numBytes []byte) (*core.Block, error) {
	if blockBytes, err := txn.Get(db.BlocksByNumber.Key(numBytes)); err != nil {
		return nil, err
	} else {
		block := new(core.Block)
		return block, cbor.Unmarshal(blockBytes, block)
	}
}

// getByHash retrieves a block from database by its hash
func getByHash(txn db.Transaction, hash *felt.Felt) (*core.Block, error) {
	if numBytes, err := txn.Get(db.BlockNumbersByHash.Key(hash.Marshal())); err != nil {
		return nil, err
	} else {
		return getByNumberBytes(txn, numBytes)
	}
}
