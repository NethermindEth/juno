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

// BlockDbKey appends hash to block number to create a db key.
// Todo: make private after Blockchain.BlockByNumber & BlockchainBlockByHash are implemented.
type BlockDbKey struct {
	Number uint64
	Hash   *felt.Felt
}

func (k *BlockDbKey) MarshalBinary() ([]byte, error) {
	var numB [8]byte
	binary.BigEndian.PutUint64(numB[:], k.Number)
	return db.Blocks.Key(numB[:], k.Hash.Marshal()), nil
}

func (k *BlockDbKey) UnmarshalBinary(data []byte) error {
	if len(data) != 41 {
		return errors.New("key should be 41 bytes long")
	}

	if data[0] != byte(db.Blocks) {
		return errors.New("wrong prefix")
	}

	k.Number = binary.BigEndian.Uint64(data[1:9])
	k.Hash = new(felt.Felt).SetBytes(data[9:41])
	return nil
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
func (b *Blockchain) Height() *uint64 {
	headBlock, err := b.Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		panic(fmt.Sprintf("failed to retrieved head block: %v", err))
	}
	if headBlock != nil {
		return &headBlock.Number
	}
	return nil
}

func (b *Blockchain) Head() (*core.Block, error) {
	txn := b.database.NewTransaction(false)
	defer txn.Discard()

	headBlockBin, err := txn.Get(db.HeadBlock.Key())
	if err != nil {
		return nil, err
	}

	headBlock := new(core.Block)
	if err = cbor.Unmarshal(headBlockBin, headBlock); err != nil {
		return nil, err
	}
	return headBlock, nil
}

// Store takes a block and state update and performs sanity checks before putting in the database.
func (b *Blockchain) Store(block *core.Block, stateUpdate *core.StateUpdate) error {
	return b.database.Update(func(txn db.Transaction) error {
		if err := b.VerifyBlock(block, stateUpdate); err != nil {
			return err
		}
		key := &BlockDbKey{block.Number, block.Hash}
		bKey, err := key.MarshalBinary()
		if err != nil {
			return err
		}

		blockBinary, err := cbor.Marshal(block)
		if err != nil {
			return err
		}

		if err = state.NewState(txn).Update(stateUpdate); err != nil {
			return err
		}

		if err = txn.Set(db.HeadBlock.Key(), blockBinary); err != nil {
			return err
		}
		return txn.Set(bKey, blockBinary)
	})
}

func (b *Blockchain) VerifyBlock(block *core.Block, stateUpdate *core.StateUpdate) error {
	/*
		Todo: Transaction and TransactionReceipts
			- When Block is changed to include a list of Transaction and TransactionReceipts
			- Further checks would need to be added to ensure Transaction Hash has been computed
				properly.
			- Sanity check would need to include checks which ensure there is same number of
				Transactions and TransactionReceipts.
	*/
	head, err := b.Head()
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

	// Todo: Fix BlockHash. For blocks 0 and 1 on mainnet the block hash which is calculated is
	// 	different from what is given by the feeder gateway.
	//	We may need to extend the unverifiable block range to account for this.
	//	There is a test for this in blockchain_test.go, once this it fixed it should be uncommented.
	//h, err := core.BlockHash(block, b.network)
	//if err != nil && !errors.As(err, new(*core.ErrUnverifiableBlock)) {
	//	return err
	//}
	//
	//if h != nil && !block.Hash.Equal(h) {
	//	return &ErrIncompatibleBlock{"incorrect block hash"}
	//}

	return nil
}
