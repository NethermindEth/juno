package db

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestNewMDBXDatabase(t *testing.T) {
	env, err := NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	_, err = NewMDBXDatabase(env, "DATABASE")
	if err != nil {
		t.Error(err)
	}
	_, err = NewMDBXDatabase(env, "DATABASE")
	if err != nil {
		t.Error(err)
	}
}

func TestMDBXDatabase_Has(t *testing.T) {
	dbs := initDatabases(t, 2)
	defer closeDatabases(dbs)
	key := []byte("key")
	value := []byte("value")
	err := dbs[0].Put(key, value)
	if err != nil {
		t.Fatal(err)
	}
	if exists, err := dbs[0].Has(key); err != nil {
		t.Errorf("unexpected error: %s", err)
	} else if !exists {
		t.Errorf("the key must exist")
	}
	if exists, err := dbs[1].Has(key); err != nil {
		t.Errorf("unexpected error: %s", err)
	} else if exists {
		t.Errorf("the key must not exist")
	}
}

func TestMDBXDatabase_Get(t *testing.T) {
	dbs := initDatabases(t, 2)
	defer closeDatabases(dbs)
	db1 := dbs[0]
	db2 := dbs[1]
	key := []byte("key")
	value1 := []byte("value1")
	value2 := []byte("value2")
	if err := db1.Put(key, value1); err != nil {
		t.Errorf("unexpected error during insert into database: %s", err)
	}
	if err := db2.Put(key, value2); err != nil {
		t.Errorf("unexpected error during insert into database: %s", err)
	}
	if outValue1, err := db1.Get(key); err != nil {
		t.Errorf("unexpected error searching the value on the database: %s", err)
	} else if bytes.Compare(outValue1, value1) != 0 {
		t.Errorf("values are different after Put & Get operations: %s, want: %s", outValue1, value2)
	}
	if outValue2, err := db2.Get(key); err != nil {
		t.Errorf("unexpected error during search into database: %s", err)
	} else if bytes.Compare(outValue2, value2) != 0 {
		t.Errorf("values are different after Put & Get operations: %s, want: %s", outValue2, value2)
	}
}

func TestMDBXDatabase_Get_NotFound(t *testing.T) {
	dbs := initDatabases(t, 1)
	defer closeDatabases(dbs)

	db := dbs[0]
	value, err := db.Get([]byte("key"))
	if err == nil || !IsNotFound(err) {
		t.Errorf("error must be an ErrNotFound")
	}
	if value != nil {
		t.Errorf("if ErrNotFound is returned, then the value must be nil")
	}
}

func TestMDBXDatabase_Delete(t *testing.T) {
	dbs := initDatabases(t, 2)
	defer closeDatabases(dbs)

	key := []byte("key")
	value := []byte("value")
	for _, db := range dbs {
		if err := db.Put(key, value); err != nil {
			t.Error(err)
		}
	}
	if err := dbs[1].Delete(key); err != nil {
		t.Error(err)
	}
	if exists, err := dbs[1].Has(key); err != nil {
		t.Error(err)
	} else if exists {
		t.Errorf("key exists after deletion")
	}
	if exists, err := dbs[0].Has(key); err != nil {
		t.Error(err)
	} else if !exists {
		t.Errorf("key does no exist")
	}
}

func TestMDBXDatabase_NumberOfItems(t *testing.T) {
	db := initDatabases(t, 1)[0]
	defer db.Close()

	assertNumberOfItems(t, db, 0)

	for i := 0; i < 10; i++ {
		err := db.Put([]byte(fmt.Sprintf("key_%d", i)), []byte("value"))
		if err != nil {
			t.Error(err)
		}
	}

	assertNumberOfItems(t, db, 10)

	for i := 0; i < 5; i++ {
		err := db.Delete([]byte(fmt.Sprintf("key_%d", i)))
		if err != nil {
			t.Error(err)
		}
	}

	assertNumberOfItems(t, db, 5)
}

func TestMDBXDatabase_RunTxn(t *testing.T) {
	dbs := initDatabases(t, 1)
	defer closeDatabases(dbs)
	db := dbs[0]
	assertNumberOfItems(t, db, 0)
	err := db.RunTxn(func(txn DatabaseOperations) error {
		assertNumberOfItems(t, txn, 0)
		if err := txn.Put([]byte("key"), []byte("value1")); err != nil {
			return err
		}
		ok, err := txn.Has([]byte("key"))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("key not fund after Put")
		}
		assertNumberOfItems(t, txn, 1)
		err = txn.Delete([]byte("key"))
		if err != nil {
			return err
		}
		assertNumberOfItems(t, txn, 0)
		return nil
	})
	if err != nil {
		t.Error(err)
	}

	assertNumberOfItems(t, db, 0)
}

func TestNewDbError(t *testing.T) {
	err := newDbError(ErrInternal, fmt.Errorf(""))
	if !errors.Is(err, ErrInternal) {
		t.Error("unexpecteed error type")
	}
}

func TestInitializeMDBXEnv(t *testing.T) {
	err := InitializeMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	if !initialized {
		t.Errorf("initialized must be true after environment initialization")
	}
}

func TestGetMDBXEnv(t *testing.T) {
	env = nil
	initialized = false
	_, err := GetMDBXEnv()
	if !errors.Is(err, ErrEnvNoInitialized) {
		t.Errorf("expected ErrEnvNoInitialized error")
	}
	err = InitializeMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	if !initialized {
		t.Errorf("initialized must be true after environment initialization")
	}
	e, err := GetMDBXEnv()
	if err != nil {
		t.Error("unexpected error")
	}
	if e != env {
		t.Error("unexpected environment")
	}
}

func initDatabases(t *testing.T, count int) []*MDBXDatabase {
	out := make([]*MDBXDatabase, count)
	env, err := NewMDBXEnv(t.TempDir(), uint64(count), 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		out[i], err = NewMDBXDatabase(env, fmt.Sprintf("db_%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func closeDatabases(dbs []*MDBXDatabase) {
	for _, db := range dbs {
		db.Close()
	}
}

func assertNumberOfItems(t *testing.T, db DatabaseOperations, expected uint64) {
	count, err := db.NumberOfItems()
	if err != nil {
		t.Error(err)
	}
	if count != expected {
		t.Errorf("unexpected number of items %d, want: %d", count, expected)
	}
}
