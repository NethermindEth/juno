package db

import (
	"testing"
)

// setupTransactionDbTest creates a new TransactionDb for Tests
func setupTransactionDbTest(database Databaser) *TransactionDb {
	return NewTransactionDb(database.GetEnv())
}

// TestAddKeyToTransaction Check that a single value is stored after made commit
func TestInsertKeyOnTransactionDbAndCommit(t *testing.T) {
	dbKV := NewKeyValueDb(t.TempDir(), 0)
	dbTest := setupTransactionDbTest(dbKV)

	database := dbTest.Begin()

	_ = database.GetEnv()

	err := database.Put([]byte("key"), []byte("value"))

	if err != nil {
		t.Log(err)
		t.Fail()
	}

	err = database.Commit()
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	database = dbTest.Begin()

	get, err := database.Get([]byte("key"))
	if err != nil || get == nil {
		t.Log(err)
		t.Fail()
	}

	if string(get) != "value" {
		t.Fail()
	}

	database.Close()
}

// TestInsertKeyOnTransactionDbAndRollback Check that a single is deleted after a rollback
func TestInsertKeyOnTransactionDbAndRollback(t *testing.T) {
	dbKV := NewKeyValueDb(t.TempDir(), 0)
	dbTest := setupTransactionDbTest(dbKV)

	database := dbTest.Begin()

	numItems, _ := database.NumberOfItems()
	if numItems != 0 {
		t.Fail()
	}

	err := database.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	numItems, _ = database.NumberOfItems()
	if numItems != 1 {
		t.Fail()
	}

	database.Rollback()

	database = dbTest.Begin()

	has, err := database.Has([]byte("key"))
	if err != nil || has {
		t.Log(err)
		t.Fail()
	}

	numItems, _ = database.NumberOfItems()
	if numItems != 0 {
		t.Fail()
	}

	database.Close()
}

// TestDeletionOnTransactionDb Check that a key is inserted and deleted properly
func TestDeletionOnTransactionDb(t *testing.T) {
	dbKV := NewKeyValueDb(t.TempDir(), 0)
	dbTest := setupTransactionDbTest(dbKV)

	database := dbTest.Begin()

	err := database.Delete([]byte("not_key"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	err = database.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	err = database.Commit()
	if err != nil {
		t.Fail()
		return
	}

	database = dbTest.Begin()

	err = database.Delete([]byte("key"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	has, err := database.Has([]byte("key"))
	if err != nil || has {
		t.Log(err)
		t.Fail()
	}

	database.Rollback()

	database = dbTest.Begin()
	has, err = database.Has([]byte("key"))
	if err != nil || !has {
		t.Log(err)
		t.Fail()
	}

	database.Close()
}
