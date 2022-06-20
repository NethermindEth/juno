package db

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNamedDatabase_Has(t *testing.T) {
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

func TestNamedDatabase_Get(t *testing.T) {
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

func TestNamedDatabase_Get_NotFound(t *testing.T) {
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

func TestNamedDatabase_Delete(t *testing.T) {
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

func TestNamedDatabase_NumberOfItems(t *testing.T) {
	db := initDatabases(t, 1)[0]
	defer db.Close()

	assertNumberOfItems := func(value uint64) {
		count, err := db.NumberOfItems()
		if err != nil {
			t.Errorf("unexpected error during getting the number of items")
		}
		if count != value {
			t.Errorf("number of items does not match, %d, want: %d", count, value)
		}
	}

	assertNumberOfItems(0)

	for i := 0; i < 10; i++ {
		err := db.Put([]byte(fmt.Sprintf("key_%d", i)), []byte("value"))
		if err != nil {
			t.Error(err)
		}
	}

	assertNumberOfItems(10)

	for i := 0; i < 5; i++ {
		err := db.Delete([]byte(fmt.Sprintf("key_%d", i)))
		if err != nil {
			t.Error(err)
		}
	}

	assertNumberOfItems(5)
}

func TestNamedDatabase_GetEnv(t *testing.T) {
	dbs := initDatabases(t, 10)
	defer closeDatabases(dbs)

	for _, db := range dbs {
		if env != db.GetEnv() {
			t.Errorf("unexpected env")
		}
	}
}

func TestNamedDatabaseTx(t *testing.T) {
	dbs := initDatabases(t, 1)
	defer closeDatabases(dbs)
	db := dbs[0]
	assertNumberOfItems(t, db, 0)
	txn, err := db.Begin()
	if err != nil {
		t.Error(err)
	}
	if env != txn.GetEnv() {
		t.Errorf("unexpected env")
	}
	assertNumberOfItems(t, txn, 0)
	if err := txn.Put([]byte("key"), []byte("value1")); err != nil {
		t.Error(err)
	}
	ok, err := txn.Has([]byte("key"))
	if err != nil {
		t.Error(err)
	}
	if !ok {
		t.Errorf("key not fund after Put")
	}
	assertNumberOfItems(t, txn, 1)
	err = txn.Delete([]byte("key"))
	if err != nil {
		t.Error(err)
	}
	assertNumberOfItems(t, txn, 0)
	if err := txn.Commit(); err != nil {
		t.Error(err)
	}
	assertNumberOfItems(t, db, 0)
}

func TestNamedDatabaseTx_Rollback(t *testing.T) {
	dbs := initDatabases(t, 1)
	defer closeDatabases(dbs)
	db := dbs[0]
	txn, err := db.Begin()
	if err != nil {
		t.Error(err)
	}
	assertNumberOfItems(t, txn, 0)
	err = txn.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Error(err)
	}
	txn.Rollback()
	assertNumberOfItems(t, db, 0)
}

func initDatabases(t *testing.T, count int) []*NamedDatabase {
	out := make([]*NamedDatabase, count)
	err := InitializeDatabaseEnv(t.TempDir(), uint64(count), 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		out[i], err = GetDatabase(fmt.Sprintf("db_%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func closeDatabases(dbs []*NamedDatabase) {
	for _, db := range dbs {
		db.Close()
	}
}

func assertNumberOfItems(t *testing.T, db Databaser, expected uint64) {
	count, err := db.NumberOfItems()
	if err != nil {
		t.Error(err)
	}
	if count != expected {
		t.Errorf("unexpected number of items %d, want: %d", count, expected)
	}
}
