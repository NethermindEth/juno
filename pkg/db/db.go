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
	// Begin starts a new transaction.
	Begin()
	// Rollback rollsback the database to a previous state.
	Rollback()
	// Close closes the environment.
	Close()
}

// KeyValueDb represents the middleware for an MDBX key-value store.
type KeyValueDb struct {
	env  *mdbx.Env
	path string
}

// New creates a new key-value database.
func New(path string, flags uint) *KeyValueDb {
	log.Default.With("Path", path, "Flags", flags).Info("Creating new database.")
	env, err := mdbx.NewEnv()
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to initialise and allocate new mdbx.Env.")
		return nil
	}

	// Set flags.
	// Based on https://github.com/torquem-ch/mdbx-go/blob/96f31f483af593377e52358a079e834256d5af55/mdbx/env_test.go#L495
	err = env.SetOption(mdbx.OptMaxDB, 1024)
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to set mdbx.Env options.")
		return nil
	}
	const pageSize = 4096
	err = env.SetGeometry(-1, -1, 64*1024*pageSize, -1, -1, pageSize)
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to set geometry.")
		return nil
	}
	err = env.Open(path, flags, 0664)
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to open mdbx.Env.")
		return nil
	}
	return &KeyValueDb{env: env, path: path}
}

// Has returns true if the value at the provided key is in the
// database.
func (d *KeyValueDb) Has(key []byte) (has bool, err error) {
	val, err := d.getOne(key)
	if err != nil {
		log.Default.With("Error", err, "Key", string(key)).Info("Key provided does not exist.")
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
			log.Default.With("Error", err).Info("Failed to open database.")
			return err
		}
		val, err = txn.Get(dbi, key)
		if err != nil {
			if mdbx.IsNotFound(err) {
				log.Default.With("Error", err, "Key", string(key)).Info("Failed to get value.")
				err = nil
				return nil
			}
			log.Default.With("Error", err, "Key", string(key)).Info("Failed to get value.")
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
	log.Default.With("Key", key).Info("Getting value using key.")
	return d.getOne(key)
}

// Put inserts a key-value pair into the database.
func (d *KeyValueDb) Put(key, value []byte) error {
	log.Default.With("Key", string(key)).Info("Putting value at key.")
	err := d.env.Update(func(txn *mdbx.Txn) error {
		log.Default.Info("Opening the root database.")
		dbi, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			log.Default.With("Error", err, "Key", string(key)).Info("Unable to open root database.")
			return err
		}
		log.Default.Info("Storing item in database.")
		return txn.Put(dbi, key, value, 0)
	})
	return err
}

// Delete removes a previous inserted key or returns an error otherwise.
func (d *KeyValueDb) Delete(key []byte) error {
	log.Default.With("Key", key).Info("Deleting value associated with key.")
	err := d.env.Update(func(txn *mdbx.Txn) error {
		db, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			log.Default.With("Error", err, "Key", string(key)).Info("Unable to open database.")
			return err
		}
		err = txn.Del(db, key, nil)
		if mdbx.IsNotFound(err) {
			log.Default.With("Error", err, "Key", string(key)).Info("Unable to get value.")
			return nil
		}
		return err
	})
	return err
}

// Count returns the number of items in the database.
func (d *KeyValueDb) Count() (uint64, error) {
	log.Default.Info("Getting the number of items in the database.")
	stats, err := d.env.Stat()
	if err != nil {
		log.Default.With("Error", err).Info("Unable to get stats from env.")
		return 0, err
	}
	return stats.Entries, err
}

// Begin starts a new transaction.
func (d KeyValueDb) Begin() {}

// Rollback rolls back the database to a previous state.
func (d KeyValueDb) Rollback() {}

// Close closes the environment.
func (d *KeyValueDb) Close() {
	d.env.Close()
}
