package db

import (
	"runtime"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

// Transactioner describes methods relating to an abstract key-value
// database oriented to transactions.
type Transactioner interface {
	// Begin starts a new transaction.
	Begin() Transaction
}

type Transactioner2 interface {
	Begin() (Transaction, error)
}

type TransactionalDb interface {
	Databaser
	Transactioner2
}

type Transaction interface {
	Databaser
	Commit() error
	Rollback()
}

type TransactionDb struct {
	env *mdbx.Env
}

type transaction struct {
	txn *mdbx.Txn
	env *mdbx.Env
}

// NewTransactionDb creates a new key-value database based on transactions.
func NewTransactionDb(env *mdbx.Env) *TransactionDb {
	return &TransactionDb{env: env}
}

func (d *TransactionDb) Begin() Transaction {
	txn, err := d.env.BeginTxn(nil, 0)
	if err != nil {
		panic(any(err))
	}
	return &transaction{txn: txn, env: d.env}
}

// GetEnv returns the environment of the database
func (d *transaction) GetEnv() *mdbx.Env {
	return d.env
}

// Has returns true if the value at the provided key is in the
// database.
func (d *transaction) Has(key []byte) (has bool, err error) {
	val, err := d.getOne(key)
	if err != nil {
		return false, err
	}
	return val != nil, nil
}

// getOne returns the value associated with the provided key in the
// database or returns an error otherwise.
func (d *transaction) getOne(key []byte) (val []byte, err error) {
	var dbi mdbx.DBI
	dbi, err = d.txn.OpenRoot(mdbx.Create)
	if err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	val, err = d.txn.Get(dbi, key)
	if err != nil {
		if mdbx.IsNotFound(err) {
			err = nil
			return nil, nil
		}
		return nil, err
	}
	runtime.UnlockOSThread()
	return val, nil
}

// Get returns the value associated with the provided key in the
// database or returns an error otherwise.
func (d *transaction) Get(key []byte) ([]byte, error) {
	return d.getOne(key)
}

// Put inserts a key-value pair into the database.
func (d *transaction) Put(key, value []byte) error {
	dbi, err := d.txn.OpenRoot(mdbx.Create)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	err = d.txn.Put(dbi, key, value, 0)
	runtime.UnlockOSThread()

	return err
}

// Delete removes a previous inserted key or returns an error otherwise.
func (d *transaction) Delete(key []byte) error {
	db, err := d.txn.OpenRoot(mdbx.Create)
	if err != nil {
		return err
	}
	err = d.txn.Del(db, key, nil)
	if mdbx.IsNotFound(err) {
		return nil
	}
	return err
}

// NumberOfItems returns the number of items in the database.
func (d *transaction) NumberOfItems() (uint64, error) {
	stats, err := d.env.Stat()
	if err != nil {
		log.Default.With("Error", err).Info("Unable to get stats from env.")
		return 0, err
	}
	return stats.Entries, err
}

// Close closes the environment.
func (d *transaction) Close() {
	d.env.Close()
}

// Commit saves all the information included in the current transaction
func (d *transaction) Commit() error {
	if d.txn != nil {
		_, err := d.txn.Commit()
		if err != nil {
			d.txn = nil
			return err
		}
	}
	return nil
}

// Rollback rolls back the database to a previous state.
func (d *transaction) Rollback() {
	if d.txn != nil {
		d.txn.Abort()
	}
}
