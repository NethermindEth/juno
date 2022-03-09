package db

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

// KeyValueDatabase represents the middleware for the MDBX database.
type Databaser interface {
	// Has returns true if the value at the provided key is in the
	// database.
	Has(key []byte) (bool, error)

	// Get returns the value associated with the provided key in the
	// database or returns an error otherwise.
	Get(key []byte) ([]byte, error)

	// Put inserts a key-value pair into the database.
	Put(key, value []byte) error

	// TODO: Document when error != nil.
	// Delete Remove a previous inserted key, otherwise nothing happen, should return
	Delete(key []byte) error

	// NumberOfItems returns the number of items in the database.
	NumberOfItems() (uint64, error)

	// XXX: Unclear. A new transaction?
	// Begin create the new
	Begin()

	// XXX: Unclear. Rolls back the database to a previous state perhaps?
	// Rollback should return
	Rollback()

	// Close closes the environment.
	Close()
}

// KVDatabase represents the middleware for an MDBX key-value store.
type KVDatabase struct {
	env  *mdbx.Env
	path string
}

// NewDatabase creates a new KVDatabase.
func NewDatabase(path string, flags uint) *KVDatabase {
	log.Default.With("Path", path, "Flags", flags).Info("Creating new database.")
	env, err := mdbx.NewEnv()
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to initialise and allocate new Env.")
		return nil
	}

	// Set flags.
	// Based on https://github.com/torquem-ch/mdbx-go/blob/96f31f483af593377e52358a079e834256d5af55/mdbx/env_test.go#L495
	err = env.SetOption(mdbx.OptMaxDB, 1024)
	if err != nil {
		// notest
		log.Default.With("Error", err).Info("Failed to set Env options.")
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
		log.Default.With("Error", err).Info("Failed to open Env.")
		return nil
	}
	return &KVDatabase{env: env, path: path}
}

// Has returns true if the value at the provided key is in the
// database.
func (d *KVDatabase) Has(key []byte) (has bool, err error) {
	val, err := d.getOne(key)
	if err != nil {
		log.Default.With("Error", err, "Key", string(key)).Info("Key provided does not exist.")
		return false, err
	}
	return val != nil, nil
}

// getOne returns the value associated with the provided key in the
// database or returns an error otherwise.
func (d *KVDatabase) getOne(key []byte) (val []byte, err error) {
	var db mdbx.DBI
	if err := d.env.View(func(txn *mdbx.Txn) error {
		db, err = txn.OpenRoot(mdbx.Create)
		if err != nil {
			log.Default.With("Error", err).Info("Failed to open database.")
			return err
		}
		val, err = txn.Get(db, key)
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
		// Log already printed in the previous call
		return nil, err
	}
	return val, err
}

// Get returns the value associated with the provided key in the
// database or returns an error otherwise.
func (d *KVDatabase) Get(key []byte) ([]byte, error) {
	log.Default.With("Key", key).Info("Getting value of provided key.")
	return d.getOne(key)
}

// Put inserts a key-value pair into the database.
func (d *KVDatabase) Put(key, value []byte) error {
	log.Default.With("Key", string(key)).Info("Putting value at provided key.")
	err := d.env.Update(func(txn *mdbx.Txn) error {
		log.Default.Info("Opening the root database.")
		dbi, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			log.Default.With("Error", err, "Key", string(key)).Info("Unable to open mdbx database")
			return err
		}
		log.Default.Info("Storing item in database.")
		return txn.Put(dbi, key, value, 0)
	})
	return err
}

// TODO: Document when error != nil.
// Delete Remove a previous inserted key, otherwise nothing happen,
// should return.
func (d *KVDatabase) Delete(key []byte) error {
	log.Default.With("Key", key).Info("Deleting value associated with provided key.")
	err := d.env.Update(func(txn *mdbx.Txn) error {
		db, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			log.Default.With("Error", err, "Key", string(key)).Info("Unable to open mdbx database")
			return err
		}
		err = txn.Del(db, key, nil)
		if mdbx.IsNotFound(err) {
			log.Default.With("Error", err, "Key", string(key)).Info("Unable to get value")
			return nil
		}
		return err
	})
	return err
}

// NumberOfItems returns the number of items in the database.
func (d *KVDatabase) NumberOfItems() (uint64, error) {
	log.Default.Info("Getting the number of items in the database.")
	stats, err := d.env.Stat()
	if err != nil {
		log.Default.With("Error", err).Info("Unable to get stats from environment")
		return 0, err
	}
	return stats.Entries, err
}

// XXX: Unclear. A new transaction?
// Begin create the new
func (d KVDatabase) Begin() {}

// XXX: Unclear. Rolls back the database to a previous state perhaps?
// Rollback should return
func (d KVDatabase) Rollback() {}

// Close environment
func (d *KVDatabase) Close() {
	d.env.Close()
}
