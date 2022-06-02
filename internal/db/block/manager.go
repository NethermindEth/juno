package block

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/internal/db"
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
func (manager *Manager) GetBlockByHash(blockHash []byte) *Block {
	// Build the hash key
	hashKey := buildHashKey(blockHash)
	// Search on the database
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		panic(any(err))
	}
	// Unmarshal the data
	block := &Block{}
	err = proto.Unmarshal(rawResult, block)
	if err != nil {
		panic(any(err))
	}
	return block
}

// GetBlockByNumber search the block with the given block number. If the block
// does not exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlockByNumber(blockNumber uint64) *Block {
	// Build the number key
	numberKey := buildNumberKey(blockNumber)
	// Search for the hash key
	hashKey, err := manager.database.Get(numberKey)
	if err != nil {
		panic(any(err))
	}
	// Search for the block
	rawResult, err := manager.database.Get(hashKey)
	if err != nil {
		panic(any(err))
	}
	// Unmarshal the data
	block := &Block{}
	err = proto.Unmarshal(rawResult, block)
	if err != nil {
		panic(any(err))
	}
	return block
}

// PutBlock saves the given block with the given hash as key. If any error happens
// then panic.
func (manager *Manager) PutBlock(blockHash []byte, block *Block) {
	// Build the keys
	hashKey := buildHashKey(blockHash)
	numberKey := buildNumberKey(block.BlockNumber)
	// Encode the block as []byte
	rawValue, err := proto.Marshal(block)
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

func buildHashKey(blockHash []byte) []byte {
	return append([]byte("blockHash:"), blockHash...)
}

func buildNumberKey(blockNumber uint64) []byte {
	numberB := make([]byte, 8)
	binary.BigEndian.PutUint64(numberB, blockNumber)
	return append([]byte("block_number:"), numberB...)
}
