package db

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/NethermindEth/juno/internal/log"

	"go.uber.org/zap"

	"github.com/torquem-ch/mdbx-go/mdbx"
)

var (
	ErrInternal = errors.New("internal error")
	ErrNotFound = errors.New("not found error")
	ErrTxInit   = errors.New("initialize transaction error")
	ErrTxCommit = errors.New("transaction commit error")
)

var (
	env    *mdbx.Env
	logger *zap.SugaredLogger
)

func InitializeDatabaseEnv(path string, optMaxDB uint64, flags uint) error {
	_env, err := mdbx.NewEnv()
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	err = _env.SetOption(mdbx.OptMaxDB, optMaxDB)
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	err = _env.SetGeometry(268435456, 268435456, 25769803776, 268435456, 268435456, 4096)
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	err = _env.Open(path, flags|mdbx.Exclusive, 0o664)
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	env = _env
	logger = log.Default.Named("Database")
	return nil
}

func GetDatabase(name string) (*NamedDatabase, error) {
	var ndb *NamedDatabase
	err := env.Update(func(txn *mdbx.Txn) error {
		databases, err := txn.ListDBI()
		if err != nil {
			// notest
			logger.Error(err)
			return fmt.Errorf("%w: %s", ErrInternal, err.Error())
		}
		for _, db := range databases {
			// Database already exists
			if db == name {
				// Open the DBI
				// notest
				dbi, err := txn.OpenDBISimple(name, 0)
				if err != nil {
					return fmt.Errorf("%w: %s", ErrInternal, err.Error())
				}
				ndb = NewNamedDatabase(name, dbi)

				return nil
			}
		}
		// Create the DBI
		dbi, err := txn.CreateDBI(name)
		if err != nil {
			// notest
			logger.With("error", err).Error("error creating a dbi")
			return fmt.Errorf("%w: %s", ErrInternal, err.Error())
		}
		logger.With("database", name).Info("new named database")
		ndb = NewNamedDatabase(name, dbi)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ndb, nil
}

type NamedDatabase struct {
	name   string
	dbi    mdbx.DBI
	logger *zap.SugaredLogger
}

func NewNamedDatabase(name string, dbi mdbx.DBI) *NamedDatabase {
	return &NamedDatabase{
		name:   name,
		dbi:    dbi,
		logger: logger.Named(name),
	}
}

func (db *NamedDatabase) Has(key []byte) (bool, error) {
	var exists bool
	err := env.View(func(txn *mdbx.Txn) error {
		var err error
		exists, err = has(txn, db.dbi, key)
		return err
	})
	return exists, err
}

func (db *NamedDatabase) Get(key []byte) ([]byte, error) {
	var value []byte
	err := env.View(func(txn *mdbx.Txn) error {
		var err error
		value, err = get(txn, db.dbi, key)
		return err
	})
	return value, err
}

func (db *NamedDatabase) Put(key, value []byte) error {
	return env.Update(func(txn *mdbx.Txn) error {
		return put(txn, db.dbi, key, value)
	})
}

func (db *NamedDatabase) Delete(key []byte) error {
	return env.Update(func(txn *mdbx.Txn) error {
		return del(txn, db.dbi, key)
	})
}

func (db *NamedDatabase) NumberOfItems() (uint64, error) {
	var entries uint64
	err := env.View(func(txn *mdbx.Txn) error {
		var err error
		entries, err = numberOfItems(txn, db.dbi)
		return err
	})
	return entries, err
}

func (db *NamedDatabase) Close() {
	env.CloseDBI(db.dbi)
}

func (db *NamedDatabase) GetEnv() *mdbx.Env {
	return env
}

func (db *NamedDatabase) Begin() (Transaction, error) {
	runtime.LockOSThread()
	txn, err := env.BeginTxn(nil, 0)
	if err != nil {
		// notest
		return nil, fmt.Errorf("%w: %s", ErrTxInit, err.Error())
	}
	return &NamedDatabaseTx{
		db:      db,
		mdbxTxn: txn,
	}, nil
}

type NamedDatabaseTx struct {
	db      *NamedDatabase
	mdbxTxn *mdbx.Txn
}

func (txn *NamedDatabaseTx) Has(key []byte) (bool, error) {
	return has(txn.mdbxTxn, txn.db.dbi, key)
}

func (txn *NamedDatabaseTx) Get(key []byte) ([]byte, error) {
	return get(txn.mdbxTxn, txn.db.dbi, key)
}

func (txn *NamedDatabaseTx) Put(key, value []byte) error {
	return put(txn.mdbxTxn, txn.db.dbi, key, value)
}

func (txn *NamedDatabaseTx) Delete(key []byte) error {
	return del(txn.mdbxTxn, txn.db.dbi, key)
}

func (txn *NamedDatabaseTx) NumberOfItems() (uint64, error) {
	return numberOfItems(txn.mdbxTxn, txn.db.dbi)
}

func (txn *NamedDatabaseTx) Close() {
	// notest
	txn.db.Close()
}

func (txn *NamedDatabaseTx) GetEnv() *mdbx.Env {
	return txn.db.GetEnv()
}

func (txn *NamedDatabaseTx) Commit() error {
	_, err := txn.mdbxTxn.Commit()
	runtime.UnlockOSThread()
	if err != nil {
		// notest
		txn.db.logger.With("error", err).Info("Commit Error")
		return fmt.Errorf("%w: %s", ErrTxCommit, err.Error())
	} else {
		txn.db.logger.Info("Commit OK")
	}
	return nil
}

func (txn *NamedDatabaseTx) Rollback() {
	txn.mdbxTxn.Abort()
	runtime.UnlockOSThread()
}

func has(txn *mdbx.Txn, dbi mdbx.DBI, key []byte) (bool, error) {
	_, err := txn.Get(dbi, key)
	if err != nil {
		if mdbx.IsNotFound(err) {
			return false, nil
		}
		// notest
		return false, fmt.Errorf("%w: %s", ErrInternal, err.Error())
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
		return nil, fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return value, nil
}

func put(txn *mdbx.Txn, dbi mdbx.DBI, key, value []byte) error {
	err := txn.Put(dbi, key, value, 0)
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return nil
}

func del(txn *mdbx.Txn, dbi mdbx.DBI, key []byte) error {
	err := txn.Del(dbi, key, nil)
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return nil
}

func numberOfItems(txn *mdbx.Txn, dbi mdbx.DBI) (uint64, error) {
	stats, err := txn.StatDBI(dbi)
	if err != nil {
		// notest
		return 0, fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return stats.Entries, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
