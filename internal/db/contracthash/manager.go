package contracthash

import (
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
)

type Manager struct {
	database db.Database
}

func NewManager(db db.Database) *Manager {
	return &Manager{database: db}
}

func (m *Manager) Close() {
	// notest
	m.database.Close()
}

func (m *Manager) GetContractHash(contractAddr string) (*felt.Felt, error) {
	// notest
	rawData, err := m.database.Get([]byte(contractAddr))
	if err != nil {
		return nil, err
	}
	return new(felt.Felt).SetBytes(rawData), nil
}

func (m *Manager) StoreContractHash(contractAddr string, contractHash *felt.Felt) error {
	return m.database.Put([]byte(contractAddr), contractHash.ByteSlice())
}
