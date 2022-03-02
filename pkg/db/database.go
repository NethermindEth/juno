package db

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

var logger = log.GetLogger()

// KeyValueDatabase Represent a middleware for mdbx key-value pair database
type KeyValueDatabase struct {
	env  *mdbx.Env
	path string
}

// NewKeyValueDatabase Creates a new KeyValueDatabase
func NewKeyValueDatabase(path string, flags uint) *KeyValueDatabase {
	logger.With(
		"Path", path,
		"Flags", flags,
	).Info("Creating new database")
	env, err1 := mdbx.NewEnv()
	if err1 != nil {
		// notest
		logger.With("Error", err1).Fatalf("Cannot create environment")
		return nil
	}
	// Set Flags
	// Based on https://github.com/torquem-ch/mdbx-go/blob/96f31f483af593377e52358a079e834256d5af55/mdbx/env_test.go#L495
	err := env.SetOption(mdbx.OptMaxDB, 1024)
	if err != nil {
		// notest
		logger.With("Error", err).Fatalf("Cannot set Options")
		return nil
	}
	const pageSize = 4096
	err = env.SetGeometry(-1, -1, 64*1024*pageSize, -1, -1, pageSize)
	if err != nil {
		// notest
		logger.With("Error", err1).Fatalf("Cannot set Geometry")
		return nil
	}
	err = env.Open(path, flags, 0664)
	if err != nil {
		// notest
		logger.With("Error", err1).Fatalf("Cannot open env")
		return nil
	}
	return &KeyValueDatabase{
		env:  env,
		path: path,
	}
}

func (d KeyValueDatabase) Has(key []byte) (has bool, err error) {
	val, err := d.getOne(key)
	if err != nil {
		return false, err
	}
	return val != nil, nil
}

func (d KeyValueDatabase) getOne(key []byte) (val []byte, err error) {
	var db mdbx.DBI
	if err := d.env.View(func(txn *mdbx.Txn) error {
		db, err = txn.OpenRoot(mdbx.Create)

		if err != nil {
			return err
		}
		val, err = txn.Get(db, key)
		if mdbx.IsNotFound(err) {
			err = nil
			return err
		}
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return val, err
}

func (d KeyValueDatabase) Get(key []byte) ([]byte, error) {
	logger.With("Key", key).Info("Getting value of provided key")
	return d.getOne(key)
}

func (d KeyValueDatabase) Put(key, value []byte) error {
	logger.With("Key", string(key)).Info("Putting value of provided key")
	err := d.env.Update(func(txn *mdbx.Txn) (err error) {
		logger.Debug("Open DBI")
		dbi, err := txn.OpenRoot(mdbx.Create)

		if err != nil {
			return err
		}
		logger.Debug("Inserting on db")
		return txn.Put(dbi, key, value, 0)
	})
	return err
}

func (d KeyValueDatabase) Delete(key []byte) error {
	logger.With("Key", key).Info("Deleting value associated to provided key")
	err := d.env.Update(func(txn *mdbx.Txn) (err error) {
		db, err := txn.OpenRoot(mdbx.Create)
		return txn.Del(db, key, nil)
	})
	return err
}

func (d KeyValueDatabase) NumberOfItems() (uint64, error) {
	logger.Info("Getting the amount of items in the collection")
	stats, err := d.env.Stat()
	if err != nil {
		return 0, err
	}
	return stats.Entries, err
}

func (d KeyValueDatabase) Begin() {
	//TODO implement me
}

func (d KeyValueDatabase) Rollback() {
	//TODO implement me
}

func (d KeyValueDatabase) Close() {
	d.env.Close()
}
