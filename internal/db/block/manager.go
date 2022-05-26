package block

import (
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

// GetBlock search the block with the given block hash. If the block does not
// exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlock(blockHash []byte) *Block {
	rawResult, err := manager.database.Get(blockHash)
	if err != nil {
		panic(any(err))
	}
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
	rawValue, err := proto.Marshal(block)
	if err != nil {
		panic(any(err))
	}
	err = manager.database.Put(blockHash, rawValue)
	if err != nil {
		panic(any(err))
	}
}

func (manager *Manager) Close() {
	manager.database.Close()
}
