package transaction

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"math/big"
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

// PutTransaction stores new transactions in the database. The TxHash field is
// the property used as the key in the database. This method does not check if
// the key already exists. In the case, that the key already exists the value
// is overwritten.
//
// Notice: Please be sure the transaction-specific fields are correctly set.
// See the DeployTransaction and InvokeFunctionTransaction doc to see more about it.
//
// This is an example code:
//
//	var manager transaction.Manager
//	// ...
//	var deployTx transaction.DeployTransaction
//	// ...
//	manager.PutTransaction(deployTx.AsTransaction())
func (m *Manager) PutTransaction(tx *Transaction) {
	rawKey := tx.TxHash.Bytes()
	rawData, err := tx.Marshal()
	if err != nil {
		log.Default.With("error", err).Panic("error marshalling Transaction")
	}
	err = m.database.Put(rawKey, rawData)
	if err != nil {
		log.Default.With("error", err).Panicf("database error")
	}
}

// GetTransaction searches in the database for the transaction associated with the
// given key (transaction hash). If the key does not exist then returns nil.
//
// See this code to know how to convert the result transaction into the
// correct transaction type
//
//	var manager transaction.Manager
//	//...
//	key, _ := new(big.Int).SetString("12c96ae3c050771689eb261c9bf78fac2580708c7f1f3d69a9647d8be59f1e1", 16)
//	tx := manager.GetTransaction(*key)
//	switch tx.TxType {
//	case transaction.TxDeploy:
//		deployTx := tx.AsDeploy()
//		// Print the constructor call data
//		fmt.Println(deployTx.ConstructorCalldata)
//	case transaction.TxInvokeFunction:
//		invokeFunctionTx := tx.AsInvokeFunction()
//		// Print the entry point selector
//		fmt.Println(invokeFunctionTx.EntryPointSelector)
//	}
func (m *Manager) GetTransaction(key big.Int) *Transaction {
	rawKey := key.Bytes()
	rawData, err := m.database.Get(rawKey)
	if err != nil {
		log.Default.With("error", err).Panicf("database error")
	}
	tx := &Transaction{}
	err = tx.Unmarshal(rawData)
	if err != nil {
		log.Default.With("error", err).Panicf("unmarshalling error")
	}
	return tx
}

// Close closes the manager, specific the associated database.
func (m *Manager) Close() {
	m.database.Close()
}
