package db

// DatabaseOperations represents all the core operations
// needed to store and search values on an key-value database.
type DatabaseOperations interface {
	Has(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
	Put(key, value []byte) error
	Delete(key []byte) error
	NumberOfItems() (uint64, error)
}

// Database represents a database behavior.
type Database interface {
	DatabaseOperations
	Close()
}

// DatabaseTransactional represents a databse with the
// posibility of make transactions.
type DatabaseTransactional interface {
	Database
	RunTxn(DatabaseTxOp) error
}

// DatabaseTxOp executes all the operations inside of the
// txn function. If the functions returns an error then
// the transaction is aborted, on another case the transaction
// is commited.
type DatabaseTxOp func(txn DatabaseOperations) error
