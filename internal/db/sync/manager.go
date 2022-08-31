package sync

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/internal/db"
)

var (
	l1ChainLengthKey = []byte("l1ChainLength")
	l2ChainLengthKey = []byte("l2ChainLength")
)

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
	return m.database.Put(l2ChainLengthKey, chainLengthBytes)
}

func (m *Manager) L1ChainLength() (uint64, error) {
	chainLengthBytes, err := m.database.Get(l1ChainLengthKey)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(chainLengthBytes), nil
}

func (m *Manager) PutL2ChainLength(length uint64) error {
	var chainLengthBytes []byte
	binary.BigEndian.PutUint64(chainLengthBytes, length)
	return m.database.Put(l2ChainLengthKey, chainLengthBytes)
}

func (m *Manager) L2ChainLength() (uint64, error) {
	chainLengthBytes, err := m.database.Get(l2ChainLengthKey)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(chainLengthBytes), nil
}

// Close closes the Manager.
func (m *Manager) Close() {
	m.database.Close()
}
