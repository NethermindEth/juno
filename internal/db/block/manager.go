package block

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Database
}

// NewManager returns a new Block manager using the given database.
func NewManager(database db.Database) *Manager {
	return &Manager{database: database}
}

// GetBlockByHash search the block with the given block hash. If the block does
// not exist then returns error. If any other type of error happens, then return error.
func (manager *Manager) GetBlockByHash(blockHash types.BlockHash) (*types.Block, error) {
	hashKey := buildHashKey(blockHash)              // Build the hash key
	rawResult, err := manager.database.Get(hashKey) // Search on the database
	if err != nil {
		return nil, fmt.Errorf("GetBlockByHash: failed get operation: %w", err)
	}
	// Check not found
	if rawResult == nil {
		// notest
		return nil, fmt.Errorf("GetBlockByHash: %w", db.ErrNotFound)
	}
	// Unmarshal the data
	block, err := unmarshalBlock(rawResult)
	if err != nil {
		return nil, fmt.Errorf("GetBlockByHash: %s: %w", db.ErrUnmarshal, err)
	}
	return block, nil
}

// GetBlockByNumber search the block with the given block number. If the block
// not exist then returns error. If any other type of error happens, then return error.
func (manager *Manager) GetBlockByNumber(blockNumber uint64) (*types.Block, error) {
	// Build the number key
	numberKey := buildNumberKey(blockNumber)
	// Search for the hash key
	hashKey, err := manager.database.Get(numberKey)
	if err != nil {
		return nil, fmt.Errorf("GetBlockByNumber: failed get (hashkey) operation: %w", err)
	}
	// Check not found
	if hashKey == nil {
		// notest
		return nil, fmt.Errorf("GetBlockByNumber: %w", db.ErrNotFound)
	}
	// Search for the block
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		return nil, fmt.Errorf("GetBlockByNumber: failed get (block) operation: %w", err)
	}
	// Check not found
	if rawResult == nil {
		// notest
		return nil, fmt.Errorf("GetBlockByNumber: %w", db.ErrNotFound)
	}
	// Unmarshal the data
	block, err := unmarshalBlock(rawResult)
	if err != nil {
		return nil, fmt.Errorf("GetBlockByNumber: %w: %s", db.ErrUnmarshal, err)
	}
	return block, nil
}

// PutBlock saves the given block with the given hash as key. If any error happens
// then return error.
func (manager *Manager) PutBlock(blockHash types.BlockHash, block *types.Block) error {
	// Build the keys
	hashKey := buildHashKey(blockHash)
	numberKey := buildNumberKey(block.BlockNumber)
	// Encode the block as []byte
	rawValue, err := marshalBlock(block)
	if err != nil {
		return fmt.Errorf("PutBlock: %s: %w", db.ErrMarshal, err)
	}
	// TODO: Use transaction?
	// Save (hashKey, block)
	err = manager.database.Put(hashKey, rawValue)
	if err != nil {
		return fmt.Errorf("PutBlock: failed put (hashkey -> rawvalue) operation: %w", err)
	}
	// Save (hashNumber, hashKey)
	err = manager.database.Put(numberKey, hashKey)
	if err != nil {
		return fmt.Errorf("PutBlock: failed put (numberkey -> hashkey) operation: %w", err)
	}
	return nil
}

func (manager *Manager) Close() {
	manager.database.Close()
}

func buildHashKey(blockHash types.BlockHash) []byte {
	return append([]byte("blockHash:"), blockHash.Bytes()...)
}

func buildNumberKey(blockNumber uint64) []byte {
	numberB := make([]byte, 8)
	binary.BigEndian.PutUint64(numberB, blockNumber)
	return append([]byte("block_number:"), numberB...)
}

func marshalBlock(block *types.Block) ([]byte, error) {
	protoBlock := Block{
		Hash:             block.BlockHash.Bytes(),
		BlockNumber:      block.BlockNumber,
		ParentBlockHash:  block.ParentHash.Bytes(),
		Status:           block.Status.String(),
		SequencerAddress: block.Sequencer.Bytes(),
		GlobalStateRoot:  block.NewRoot.Bytes(),
		OldRoot:          block.OldRoot.Bytes(),
		AcceptedTime:     block.AcceptedTime,
		TimeStamp:        block.TimeStamp,
		TxCount:          block.TxCount,
		TxCommitment:     block.TxCommitment.Bytes(),
		TxHashes:         marshalBlockTxHashes(block.TxHashes),
		EventCount:       block.EventCount,
		EventCommitment:  block.EventCommitment.Bytes(),
	}
	return proto.Marshal(&protoBlock)
}

func marshalBlockTxHashes(txHashes []types.TransactionHash) [][]byte {
	out := make([][]byte, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = txHash.Bytes()
	}
	return out
}

func unmarshalBlock(data []byte) (*types.Block, error) {
	var protoBlock Block
	err := proto.Unmarshal(data, &protoBlock)
	if err != nil {
		return nil, err
	}
	block := types.Block{
		BlockHash:       types.BytesToBlockHash(protoBlock.Hash),
		ParentHash:      types.BytesToBlockHash(protoBlock.ParentBlockHash),
		BlockNumber:     protoBlock.BlockNumber,
		Status:          types.StringToBlockStatus(protoBlock.Status),
		Sequencer:       types.BytesToAddress(protoBlock.SequencerAddress),
		NewRoot:         types.BytesToFelt(protoBlock.GlobalStateRoot),
		OldRoot:         types.BytesToFelt(protoBlock.OldRoot),
		AcceptedTime:    protoBlock.AcceptedTime,
		TimeStamp:       protoBlock.TimeStamp,
		TxCount:         protoBlock.TxCount,
		TxCommitment:    types.BytesToFelt(protoBlock.TxCommitment),
		TxHashes:        unmarshalBlockTxHashes(protoBlock.TxHashes),
		EventCount:      protoBlock.EventCount,
		EventCommitment: types.BytesToFelt(protoBlock.EventCommitment),
	}
	return &block, nil
}

func unmarshalBlockTxHashes(txHashes [][]byte) []types.TransactionHash {
	out := make([]types.TransactionHash, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = types.BytesToTransactionHash(txHash)
	}
	return out
}
