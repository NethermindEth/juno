package sync

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/internal/db"
)

var chainLengthKey = []byte("chainLength")

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Database
}

// NewManager returns a new Block manager using the given database.
func NewManager(database db.Database) *Manager {
	return &Manager{database: database}
}

func (m *Manager) PutL1ChainLength(length uint64) error {
	var chainLengthBytes []byte
	binary.BigEndian.PutUint64(chainLengthBytes, length)
	return m.database.Put(chainLengthKey, chainLengthBytes)
}

func (m *Manager) L1ChainLength() (uint64, error) {
	chainLengthBytes, err := m.database.Get(chainLengthKey)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(chainLengthBytes), nil
}
