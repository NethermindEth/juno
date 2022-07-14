package block

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
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
// not exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlockByHash(blockHash *felt.Felt) (*types.Block, error) {
	// Build the hash key
	hashKey := buildHashKey(blockHash)
	// Search on the database
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		return nil, err
	}
	// Unmarshal the data
	block, err := unmarshalBlock(rawResult)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// GetBlockByNumber search the block with the given block number. If the block
// does not exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlockByNumber(blockNumber uint64) (*types.Block, error) {
	// Build the number key
	numberKey := buildNumberKey(blockNumber)
	// Search for the hash key
	hashKey, err := manager.database.Get(numberKey)
	if err != nil {
		return nil, err
	}
	// Search for the block
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		return nil, err
	}
	// Unmarshal the data
	block, err := unmarshalBlock(rawResult)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// PutBlock saves the given block with the given hash as key. If any error happens
// then panic.
func (manager *Manager) PutBlock(blockHash *felt.Felt, block *types.Block) error {
	// Build the keys
	hashKey := buildHashKey(blockHash)
	numberKey := buildNumberKey(block.BlockNumber)
	// Encode the block as []byte
	rawValue, err := marshalBlock(block)
	if err != nil {
		return err
	}
	// TODO: Use transaction?
	// Save (hashKey, block)
	err = manager.database.Put(hashKey, rawValue)
	if err != nil {
		return err
	}
	// Save (hashNumber, hashKey)
	err = manager.database.Put(numberKey, hashKey)
	if err != nil {
		return err
	}
	return nil
}

func (manager *Manager) Close() {
	manager.database.Close()
}

func buildHashKey(blockHash *felt.Felt) []byte {
	return append([]byte("blockHash:"), blockHash.ByteSlice()...)
}

func buildNumberKey(blockNumber uint64) []byte {
	numberB := make([]byte, 8)
	binary.BigEndian.PutUint64(numberB, blockNumber)
	return append([]byte("block_number:"), numberB...)
}

func marshalBlock(block *types.Block) ([]byte, error) {
	protoBlock := Block{
		Hash:             block.BlockHash.ByteSlice(),
		BlockNumber:      block.BlockNumber,
		ParentBlockHash:  block.ParentHash.ByteSlice(),
		Status:           block.Status.String(),
		SequencerAddress: block.Sequencer.ByteSlice(),
		GlobalStateRoot:  block.NewRoot.ByteSlice(),
		OldRoot:          block.OldRoot.ByteSlice(),
		AcceptedTime:     block.AcceptedTime,
		TimeStamp:        block.TimeStamp,
		TxCount:          block.TxCount,
		TxCommitment:     block.TxCommitment.ByteSlice(),
		TxHashes:         marshalBlockTxHashes(block.TxHashes),
		EventCount:       block.EventCount,
		EventCommitment:  block.EventCommitment.ByteSlice(),
	}
	return proto.Marshal(&protoBlock)
}

func marshalBlockTxHashes(txHashes []*felt.Felt) [][]byte {
	out := make([][]byte, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = txHash.ByteSlice()
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
		BlockHash:       new(felt.Felt).SetBytes(protoBlock.Hash),
		ParentHash:      new(felt.Felt).SetBytes(protoBlock.ParentBlockHash),
		BlockNumber:     protoBlock.BlockNumber,
		Status:          types.StringToBlockStatus(protoBlock.Status),
		Sequencer:       new(felt.Felt).SetBytes(protoBlock.SequencerAddress),
		NewRoot:         new(felt.Felt).SetBytes(protoBlock.GlobalStateRoot),
		OldRoot:         new(felt.Felt).SetBytes(protoBlock.OldRoot),
		AcceptedTime:    protoBlock.AcceptedTime,
		TimeStamp:       protoBlock.TimeStamp,
		TxCount:         protoBlock.TxCount,
		TxCommitment:    new(felt.Felt).SetBytes(protoBlock.TxCommitment),
		TxHashes:        unmarshalBlockTxHashes(protoBlock.TxHashes),
		EventCount:      protoBlock.EventCount,
		EventCommitment: new(felt.Felt).SetBytes(protoBlock.EventCommitment),
	}
	return &block, nil
}

func unmarshalBlockTxHashes(txHashes [][]byte) []*felt.Felt {
	out := make([]*felt.Felt, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = new(felt.Felt).SetBytes(txHash)
	}
	return out
}
