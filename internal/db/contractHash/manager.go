package contractHash

import (
	"github.com/NethermindEth/juno/internal/db"
	"math/big"
)

type Manager struct {
	database db.Database
}

func NewManager(db db.Database) *Manager {
	return &Manager{database: db}
}

func (m *Manager) Close() {
	m.database.Close()
}

func (m *Manager) GetContractHash(contractAddr string) *big.Int {
	rawData, err := m.database.Get([]byte(contractAddr))
	if err != nil {
		return nil
	}
	return new(big.Int).SetBytes(rawData)
}

func (m *Manager) StoreContractHash(contractAddr string, contractHash *big.Int) error {
	return m.database.Put([]byte(contractAddr), contractHash.Bytes())
}
