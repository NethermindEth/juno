// TODO: Document.
// XXX: Why are all the method calls copying instead of referencing the
// database?
package db

import (
	"github.com/NethermindEth/juno/internal/log"
	"github.com/torquem-ch/mdbx-go/mdbx"
)

// TODO: Implement.
func (d Database) Begin()    {}
func (d Database) Rollback() {}

// Database represents the middleware for an MDBX key-value store.
type Database struct {
	env  *mdbx.Env
	path string
}

// NewDatabase creates a new Database.
func NewDatabase(path string, flags uint) *Database {
	log.Default.With("Path", path, "Flags", flags).Info("Creating new database.")
	env, err := mdbx.NewEnv()
	// TODO: Handle error using errpkg.CheckFatal.
	if err != nil {
		// notest
		log.Default.With("Error", err).Fatal("Failed to initialise and allocate new Env.")
		// TODO: The following return is superficial given that the line
		// above terminates this process.
		return nil
	}

	// Set flags.
	// Based on https://github.com/torquem-ch/mdbx-go/blob/96f31f483af593377e52358a079e834256d5af55/mdbx/env_test.go#L495
	err = env.SetOption(mdbx.OptMaxDB, 1024)
	// TODO: See above (line 25-32).
	if err != nil {
		// notest
		log.Default.With("Error", err).Fatal("Failed to set Env options.")
		return nil
	}
	const pageSize = 4096
	err = env.SetGeometry(-1, -1, 64*1024*pageSize, -1, -1, pageSize)
	// TODO: See above (line 25-32).
	if err != nil {
		// notest
		log.Default.With("Error", err).Fatal("Failed to set geometry.")
		return nil
	}
	err = env.Open(path, flags, 0664)
	// TODO: See above (line 25-32).
	if err != nil {
		// notest
		log.Default.With("Error", err).Fatal("Failed to open Env.")
		return nil
	}
	return &Database{env: env, path: path}
}

// TODO: Document.
func (d Database) Has(key []byte) (has bool, err error) {
	val, err := d.getOne(key)
	if err != nil {
		// TODO: Log error.
		return false, err
	}
	return val != nil, nil
	// XXX: If it can be guaranteed that Database.getOne will only return
	// a value in the absence of an error then we can avoid the assignment
	// and comparison as follows which also highlights intension better.
	// _, err = d.getOne(key)
	// if err != nil {
	// 	return false, err
	// }
	// return true, nil
}

// TODO: Document.
func (d Database) getOne(key []byte) (val []byte, err error) {
	var db mdbx.DBI
	if err := d.env.View(func(txn *mdbx.Txn) error {
		db, err = txn.OpenRoot(mdbx.Create)
		if err != nil {
			// TODO: Log error.
			return err
		}
		val, err = txn.Get(db, key)
		if err != nil {
			if mdbx.IsNotFound(err) {
				// TODO: Log error.
				// XXX: Why is this error not propagated?
				return nil
			}
			// TODO: Log error.
			return err
		}
		return nil
	}); err != nil {
		// TODO: Log error. But it is unclear whether this is the desired
		// behaviour given the "not found" error is not propagated.
		return nil, err
	}
	return val, err
}

// TODO: Document.
func (d Database) Get(key []byte) ([]byte, error) {
	log.Default.With("Key", key).Info("Getting value of provided key.")
	return d.getOne(key)
}

// TODO: Document.
func (d Database) Put(key, value []byte) error {
	log.Default.With("Key", string(key)).Info("Putting value at provided key.")
	err := d.env.Update(func(txn *mdbx.Txn) error {
		// XXX: Why did the logging level switch from Info to Debug?
		log.Default.Debug("Opening the root database.")
		dbi, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			// TODO: Log error.
			return err
		}
		log.Default.Debug("Storing item in database.")
		return txn.Put(dbi, key, value, 0)
	})
	return err
}

// TODO: Document.
func (d Database) Delete(key []byte) error {
	log.Default.With("Key", key).Info("Deleting value associated with provided key.")
	err := d.env.Update(func(txn *mdbx.Txn) error {
		db, err := txn.OpenRoot(mdbx.Create)
		if err != nil {
			// TODO: Log error.
			return err
		}
		return txn.Del(db, key, nil)
	})
	return err
}

// TODO: Document.
func (d Database) NumberOfItems() (uint64, error) {
	log.Default.Info("Getting the number of items in the database.")
	stats, err := d.env.Stat()
	if err != nil {
		// TODO: Log error.
		return 0, err
	}
	return stats.Entries, err
}

// TODO: Document.
func (d Database) Close() {
	d.env.Close()
}
