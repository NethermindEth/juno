package db

import (
	"fmt"
	"github.com/torquem-ch/mdbx-go/mdbx"
	"log"
)

type Database struct {
	env  *mdbx.Env
	path string
	//tempDirMu  sync.Mutex
}

func NewDatabase(path string) Database {
	env, err1 := mdbx.NewEnv()
	if err1 != nil {
		log.Fatalf("Cannot create environment: %s", err1)
	}
	return Database{
		env:  env,
		path: path,
	}
}

func (d Database) Has(key []byte) (has bool, err error) {
	err1 := d.env.Open(d.path, 0, 0664)
	defer d.env.Close()
	if err1 != nil {
		log.Fatalf("Cannot open environment: %s", err1)
		return false, err1
	}
	var db mdbx.DBI
	has = false
	if err := d.env.View(func(txn *mdbx.Txn) error {
		db, err = txn.OpenDBISimple(d.path, 0)
		if err != nil {
			return err
		}
		cursor, err := txn.OpenCursor(db)
		if err != nil {
			cursor.Close()
			return fmt.Errorf("cursor: %v", err)
		}
		var bNumVal int
		for {
			_, _, err = cursor.Get(key, nil, mdbx.Next)
			if mdbx.IsNotFound(err) {
				break
			}
			if err != nil {
				has = false
				return err
			}
			has = true
			bNumVal++
		}
		cursor.Close()
		return err
	}); err != nil {
		log.Fatal(err)
		return has, err
	}
	return has, nil
}

func (d Database) GetOne(key []byte) (val []byte, err error) {
	var db mdbx.DBI
	if err := d.env.View(func(txn *mdbx.Txn) error {
		db, err = txn.OpenDBISimple(d.path, 0)
		if err != nil {
			return err
		}
		val, err = txn.Get(db, key)
		if err != nil {
			return fmt.Errorf("get: %v", err)
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
	return val, err
}

func (d Database) Get(key []byte) ([]byte, error) {
	return d.GetOne(key)
}

func (d Database) Put(key, value []byte) error {
	err := d.env.Update(func(txn *mdbx.Txn) (err error) {
		dbi, err := txn.OpenDBI(d.path, mdbx.Create, nil, nil)
		if err != nil {
			return err
		}
		return txn.Put(dbi, key, value, 0)
	})
	return err
}

func (d Database) Delete(k, v []byte) error {
	err := d.env.Update(func(txn *mdbx.Txn) (err error) {
		db, err := txn.OpenDBISimple(d.path, 0)
		return txn.Del(db, k, v)
	})
	return err
}

func (d Database) Begin() {
	//TODO implement me
	panic("implement me")
}

func (d Database) Rollback() {
	//TODO implement me
	panic("implement me")
}

func (d Database) Close() {
	//TODO implement me
	panic("implement me")
}
