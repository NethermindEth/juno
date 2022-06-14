package db

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/log"

	"go.uber.org/zap"

	"github.com/torquem-ch/mdbx-go/mdbx"
)

var (
	ErrInternal = errors.New("internal error")
	ErrNotFound = errors.New("not found error")
)

var (
	env    *mdbx.Env
	logger *zap.SugaredLogger
)

type NamedDatabase struct {
	name   string
	dbi    mdbx.DBI
	env    *mdbx.Env
	logger *zap.SugaredLogger
}

func NewNamedDatabase(name string, dbi mdbx.DBI) *NamedDatabase {
	return &NamedDatabase{
		name:   name,
		dbi:    dbi,
		env:    env,
		logger: logger.Named(name),
	}
}

func (db *NamedDatabase) Has(key []byte) (bool, error) {
	var exists bool
	err := db.env.View(func(txn *mdbx.Txn) error {
		_, err := txn.Get(db.dbi, key)
		if err != nil {
			if mdbx.IsNotFound(err) {
				exists = false
				return nil
			}
			return err
		} else {
			exists = true
		}
		return nil
	})
	if err != nil {
		// notest
		return false, fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return exists, nil
}

func (db *NamedDatabase) Get(key []byte) ([]byte, error) {
	var value []byte
	err := db.env.View(func(txn *mdbx.Txn) error {
		_value, err := txn.Get(db.dbi, key)
		if err != nil {
			return err
		}
		value = _value
		return nil
	})
	if err != nil {
		if mdbx.IsNotFound(err) {
			return nil, ErrNotFound
		}
		// notest
		return nil, fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return value, nil
}

func (db *NamedDatabase) Put(key, value []byte) error {
	err := db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Put(db.dbi, key, value, 0)
	})
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return nil
}

func (db *NamedDatabase) Delete(key []byte) error {
	err := db.env.Update(func(txn *mdbx.Txn) error {
		return txn.Del(db.dbi, key, nil)
	})
	if err != nil {
		// notest
		return fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return nil
}

func (db *NamedDatabase) NumberOfItems() (uint64, error) {
	var numberOfItems uint64
	err := db.env.View(func(txn *mdbx.Txn) error {
		stats, err := txn.StatDBI(db.dbi)
		if err != nil {
			return err
		}
		numberOfItems = stats.Entries
		return nil
	})
	if err != nil {
		// notest
		return 0, fmt.Errorf("%w: %s", ErrInternal, err.Error())
	}
	return numberOfItems, nil
}

func (db *NamedDatabase) Close() {
	db.env.CloseDBI(db.dbi)
}

func (db *NamedDatabase) GetEnv() *mdbx.Env {
	return db.env
}

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

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
