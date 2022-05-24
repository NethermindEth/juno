package transaction

import (
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"google.golang.org/protobuf/proto"
)

// Manager manages all the related to the database of Transactions. All the
// communications with the transactions' database must be made with this manager.
// Transactions can have two types: DeployTransaction and InvokeFunctionTransaction.
type Manager struct {
	database db.Databaser
}

// NewManager returns a new instance of the Manager.
func NewManager(database db.Databaser) *Manager {
	return &Manager{database: database}
}

// PutTransaction stores new transactions in the database. This method does not
// check if the key already exists. In the case, that the key already exists the
// value is overwritten.
func (m *Manager) PutTransaction(key []byte, tx *Transaction) {
	rawData, err := proto.Marshal(tx)
	if err != nil {
		// notest
		log.Default.With("error", err).Panic("error marshalling Transaction")
	}
	err = m.database.Put(key, rawData)
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("database error")
	}
}

// GetTransaction searches in the database for the transaction associated with the
// given key. If the key does not exist then returns nil.
func (m *Manager) GetTransaction(key []byte) *Transaction {
	rawData, err := m.database.Get(key)
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("database error")
	}
	tx := new(Transaction)
	err = proto.Unmarshal(rawData, tx)
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("unmarshalling error")
	}
	return tx
}

// Close closes the manager, specific the associated database.
func (m *Manager) Close() {
	m.database.Close()
}
