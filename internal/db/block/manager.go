package block

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Databaser
}

// NewManager returns a new Block manager using the given database.
func NewManager(database db.Databaser) *Manager {
	return &Manager{database: database}
}

// GetBlockByHash search the block with the given block hash. If the block does
// not exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlockByHash(blockHash types.PedersenHash) *types.Block {
	// Build the hash key
	hashKey := buildHashKey(blockHash)
	// Search on the database
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		panic(any(err))
	}
	// Check not found
	if rawResult == nil {
		// notest
		return nil
	}
	// Unmarshal the data
	block, err := unmarshalBlock(rawResult)
	if err != nil {
		panic(any(err))
	}
	return block
}

// GetBlockByNumber search the block with the given block number. If the block
// does not exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlockByNumber(blockNumber uint64) *types.Block {
	// Build the number key
	numberKey := buildNumberKey(blockNumber)
	// Search for the hash key
	hashKey, err := manager.database.Get(numberKey)
	if err != nil {
		panic(any(err))
	}
	// Check not found
	if hashKey == nil {
		// notest
		return nil
	}
	// Search for the block
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		panic(any(err))
	}
	// Check not found
	if rawResult == nil {
		// notest
		return nil
	}
	// Unmarshal the data
	block, err := unmarshalBlock(rawResult)
	if err != nil {
		panic(any(err))
	}
	return block
}

// PutBlock saves the given block with the given hash as key. If any error happens
// then panic.
func (manager *Manager) PutBlock(blockHash types.PedersenHash, block *types.Block) {
	// Build the keys
	hashKey := buildHashKey(blockHash)
	numberKey := buildNumberKey(block.BlockNumber)
	// Encode the block as []byte
	rawValue, err := marshalBlock(block)
	if err != nil {
		panic(any(err))
	}
	// TODO: Use transaction?
	// Save (hashKey, block)
	err = manager.database.Put(hashKey, rawValue)
	if err != nil {
		panic(any(err))
	}
	// Save (hashNumber, hashKey)
	err = manager.database.Put(numberKey, hashKey)
	if err != nil {
		panic(any(err))
	}
}

func (manager *Manager) Close() {
	manager.database.Close()
}

func buildHashKey(blockHash types.PedersenHash) []byte {
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

func marshalBlockTxHashes(txHashes []types.PedersenHash) [][]byte {
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
		BlockHash:       types.BytesToPedersenHash(protoBlock.Hash),
		ParentHash:      types.BytesToPedersenHash(protoBlock.ParentBlockHash),
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

func unmarshalBlockTxHashes(txHashes [][]byte) []types.PedersenHash {
	out := make([]types.PedersenHash, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = types.BytesToPedersenHash(txHash)
	}
	return out
}
