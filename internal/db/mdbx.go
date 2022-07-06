package db

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/torquem-ch/mdbx-go/mdbx"
)

var (
	// ErrInternal represents an internal database error.
	ErrInternal = errors.New("internal error")
	// ErrNotFound is used when a key is not found in the database.
	ErrNotFound = errors.New("not found error")
	// ErrTx is returned when a transaction fails for some reason.
	ErrTx = errors.New("transaction error")
)

// MDBXDatabase is a Database that works with the LMDB database. A database is a
// named database (isolated collection stored on the same file) of a certain
// environment (database file). One environment can have many named databases.
// The environment must be initialized and configured with the correct number of
// named databases before creating the MDBXDatabase instances.
type MDBXDatabase struct {
	env *mdbx.Env
	dbi mdbx.DBI
}

// NewMDBXDatabaseWithEnv creates a new MDBXDatabase with the given environment and name.
func NewMDBXDatabaseWithEnv(env *mdbx.Env, name string) (*MDBXDatabase, error) {
	var dbi mdbx.DBI
	// Start a READ transaction
	err := env.Update(func(txn *mdbx.Txn) error {
		// Get all the current DBI
		dbis, err := txn.ListDBI()
		if err != nil {
			return newDbError(ErrInternal, err)
		}
		// Search if already exists a DBI with the given name
		for _, dbiName := range dbis {
			if dbiName == name {
				dbi, err = openDBI(txn, name)
				if err != nil {
					return newDbError(ErrInternal, err)
				}
				return nil
			}
		}
		// Create a new DBI if it does not exist
		dbi, err = txn.CreateDBI(name)
		if err != nil {
			return newDbError(ErrInternal, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &MDBXDatabase{
		env: env,
		dbi: dbi,
	}, nil
}

// Has searches on the database if the key already exists.
func (x *MDBXDatabase) Has(key []byte) (bool, error) {
	var exists bool

	// Start READ transaction
	err := x.env.View(func(txn *mdbx.Txn) error {
		var err error
		// Search for the key on the DBI
		exists, err = has(txn, x.dbi, key)
		return err
	})

	return exists, err
}

// Get returns the associated value with the given key. If the key does not
// exist it returns an ErrNotFound.
func (x *MDBXDatabase) Get(key []byte) ([]byte, error) {
	var value []byte
	// Start a READ transaction
	err := x.env.View(func(txn *mdbx.Txn) error {
		var err error
		// Search for the key value on the DBI
		value, err = get(txn, x.dbi, key)
		return err
	})
	return value, err
}

// Put store/override the given value on the given key.
func (x *MDBXDatabase) Put(key, val []byte) error {
	// Start a READ-WRITE transaction
	return x.env.Update(func(txn *mdbx.Txn) error {
		// Put the key-value pair on the DBI
		return put(txn, x.dbi, key, val)
	})
}

// Delete deletes the given key and its value.
func (x *MDBXDatabase) Delete(key []byte) error {
	// Start a READ-WRITE transaction
	return x.env.Update(func(txn *mdbx.Txn) error {
		// Delete the key-value pair on the DBI
		return del(txn, x.dbi, key)
	})
}

// NumberOfItems returns the number of keys on the database.
func (x *MDBXDatabase) NumberOfItems() (uint64, error) {
	var entries uint64

	// Start READ transaction
	err := x.env.View(func(txn *mdbx.Txn) error {
		var err error
		// Get the number of items on the DBI
		entries, err = numberOfItems(txn, x.dbi)
		return err
	})

	return entries, err
}

// Close closes the database. Notice this function does not close the
// environment.
func (x *MDBXDatabase) Close() {
	x.env.CloseDBI(x.dbi)
}

// RunTxn runs a function on a database transaction. If the function returns an
// error then the transaction is aborted. To guarantee the transaction is
// thread-safe, the function calls runtime.LockOSThread at the start and
// runtime.UnlockOSThread at the end.
func (x *MDBXDatabase) RunTxn(op DatabaseTxOp) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Initialize the transaction
	mdbxTxn, err := x.env.BeginTxn(nil, 0)
	if err != nil {
		return newDbError(ErrInternal, err)
	}
	txn := MDBXTransaction{
		txn: mdbxTxn,
		dbi: x.dbi,
	}
	// Run the operations
	err = op(txn)
	if err != nil {
		// Abort if error
		mdbxTxn.Abort()
		return newDbError(ErrTx, err)
	}
	// Commit
	_, err = mdbxTxn.Commit()
	if err != nil {
		return newDbError(ErrTx, err)
	}
	return nil
}

// MDBXTransaction represents a transaction of the MDBXDatabase.
type MDBXTransaction struct {
	txn *mdbx.Txn
	dbi mdbx.DBI
}

// Has searches on the database if the key already exists.
func (tx MDBXTransaction) Has(key []byte) (bool, error) {
	return has(tx.txn, tx.dbi, key)
}

// Get returns the associated value with the given key. If the key does not
// exist the returns ErrNotFound.
func (tx MDBXTransaction) Get(key []byte) ([]byte, error) {
	return get(tx.txn, tx.dbi, key)
}

// Put store/override the given value on the given key.
func (tx MDBXTransaction) Put(key, val []byte) error {
	return put(tx.txn, tx.dbi, key, val)
}

// Delete deletes the given key and its value.
func (tx MDBXTransaction) Delete(key []byte) error {
	return del(tx.txn, tx.dbi, key)
}

func (tx MDBXTransaction) NumberOfItems() (uint64, error) {
	return numberOfItems(tx.txn, tx.dbi)
}

// IsNotFound checks is the given error is an ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func has(txn *mdbx.Txn, dbi mdbx.DBI, key []byte) (bool, error) {
	_, err := txn.Get(dbi, key)
	if err != nil {
		if mdbx.IsNotFound(err) {
			return false, nil
		}
		// notest
		return false, newDbError(ErrInternal, err)
	}
	return true, nil
}

func get(txn *mdbx.Txn, dbi mdbx.DBI, key []byte) ([]byte, error) {
	value, err := txn.Get(dbi, key)
	if err != nil {
		if mdbx.IsNotFound(err) {
			return nil, ErrNotFound
		}
		// notest
		return nil, newDbError(ErrInternal, err)
	}
	return value, nil
}

func put(txn *mdbx.Txn, dbi mdbx.DBI, key, value []byte) error {
	err := txn.Put(dbi, key, value, 0)
	if err != nil {
		// notest
		return newDbError(ErrInternal, err)
	}
	return nil
}

func del(txn *mdbx.Txn, dbi mdbx.DBI, key []byte) error {
	err := txn.Del(dbi, key, nil)
	if err != nil {
		// notest
		return newDbError(ErrInternal, err)
	}
	return nil
}

func numberOfItems(txn *mdbx.Txn, dbi mdbx.DBI) (uint64, error) {
	stats, err := txn.StatDBI(dbi)
	if err != nil {
		// notest
		return 0, newDbError(ErrInternal, err)
	}
	return stats.Entries, nil
}

func openDBI(txn *mdbx.Txn, name string) (mdbx.DBI, error) {
	dbi, err := txn.OpenDBISimple(name, 0)
	if err != nil {
		return 0, newDbError(ErrInternal, err)
	}
	return dbi, nil
}

func newDbError(dbError, err error) error {
	return fmt.Errorf("%w: %s", dbError, err)
}
