// Package db provides a key-value database and functions related to
// operating one.
package db

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

// Databaser describes methods relating to an abstract key-value
// database.
type Databaser interface {
	// Has returns true if the value at the provided key is in the
	// database.
	Has(key []byte) (bool, error)
	// Get returns the value associated with the provided key in the
	// database or returns an error otherwise.
	Get(key []byte) ([]byte, error)
	// Put inserts a key-value pair into the database.
	Put(key, value []byte) error
	// Delete removes a previous inserted key or returns an error
	// otherwise.
	Delete(key []byte) error
	// NumberOfItems returns the number of items in the database.
	NumberOfItems() (uint64, error)
	// Close closes the environment.
	Close()
	// GetEnv returns the environment of the database
	GetEnv() *mdbx.Env
}

// KeyValueDb represents the middleware for an MDBX key-value store.
type KeyValueDb struct {
	env  *mdbx.Env
	path string
}

// GetEnv returns the environment of the database
func (d *KeyValueDb) GetEnv() *mdbx.Env {
	return d.env
}

func NewKeyValueDb(path string, flags uint) *KeyValueDb {
	env, err := mdbx.NewEnv()
	if err != nil {
		// notest
		return nil
	}

	// Set flags.
	// Based on https://github.com/torquem-ch/mdbx-go/blob/96f31f483af593377e52358a079e834256d5af55/mdbx/env_test.go#L495
	err = env.SetOption(mdbx.OptMaxDB, 1)
	if err != nil {
		// notest
		return nil
	}
	const pageSize = 4096
	err = env.SetGeometry(268435456, 268435456, 1025769803776, 268435456, 268435456, pageSize)
	if err != nil {
		// notest
		return nil
	}
	err = env.Open(path, flags|mdbx.Exclusive, 0664)
	if err != nil {
		// notest
		log.Default.With("Error", err).Panic("Couldn't open db")
		return nil
	}
	return &KeyValueDb{env: env, path: path}
}

// Has returns true if the value at the provided key is in the
// database.
func (d *KeyValueDb) Has(key []byte) (has bool, err error) {
	val, err := d.getOne(key)
	if err != nil {
		return false, err
	}
	return val != nil, nil
}

// getOne returns the value associated with the provided key in the
// database or returns an error otherwise.
func (d *KeyValueDb) getOne(key []byte) (val []byte, err error) {
	var dbi mdbx.DBI
	if err := d.env.View(func(txn *mdbx.Txn) error {
		dbi, err = txn.OpenRoot(mdbx.Create)
		if err != nil {
			return err
		}
		val, err = txn.Get(dbi, key)
		if err != nil {
			if mdbx.IsNotFound(err) {
				err = nil
				return nil
			}
			return err
		}
		return nil
	}); err != nil {
		// Log already printed in the previous call.
		return nil, err
	}
	return val, err
}

// Get returns the value associated with the provided key in the
// database or returns an error otherwise.
func (d *KeyValueDb) Get(key []byte) ([]byte, error) {
	return d.getOne(key)
}

// Put inserts a key-value pair into the database.
func (d *KeyValueDb) Put(key, value []byte) error {
	err := d.env.Update(func(txn *mdbx.Txn) error {
		dbi, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			return err
		}
		return txn.Put(dbi, key, value, 0)
	})
	return err
}

// Delete removes a previous inserted key or returns an error otherwise.
func (d *KeyValueDb) Delete(key []byte) error {
	err := d.env.Update(func(txn *mdbx.Txn) error {
		db, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			return err
		}
		err = txn.Del(db, key, nil)
		if mdbx.IsNotFound(err) {
			return nil
		}
		return err
	})
	return err
}

// NumberOfItems returns the number of items in the database.
func (d *KeyValueDb) NumberOfItems() (uint64, error) {
	stats, err := d.env.Stat()
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Unable to get stats from env.")
		return 0, err
	}
	return stats.Entries, err
}

// Close closes the environment.
func (d *KeyValueDb) Close() {
	d.env.Close()
}
