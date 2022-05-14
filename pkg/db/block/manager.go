package block

import "github.com/NethermindEth/juno/pkg/db"

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Databaser
}

// NewManager returns a new Block manager using the given database.
func NewManager(databaser db.Databaser) *Manager {
	return &Manager{database: databaser}
}

// GetBlock search the block with the given block hash. If the block does not
// exist then returns nil. If any error happens, then panic.
func (manager *Manager) GetBlock(hash BlockHashKey) *BlockValue {
	key, err := hash.Marshal()
	if err != nil {
		panic(any(err))
	}
	rawResult, err := manager.database.Get(key)
	if err != nil {
		panic(any(err))
	}
	var block BlockValue
	err = block.Unmarshal(rawResult)
	if err != nil {
		panic(any(err))
	}
	return &block
}

// PutBlock saves the given block with the given hash as key. If any error happens
// then panic.
func (manager *Manager) PutBlock(hash BlockHashKey, block BlockValue) {
	key, err := hash.Marshal()
	if err != nil {
		panic(any(err))
	}
	rawValue, err := block.Marshal()
	if err != nil {
		panic(any(err))
	}
	err = manager.database.Put(key, rawValue)
	if err != nil {
		panic(any(err))
	}
}
