package db

import (
	"testing"
)

// setupTransactionDbTest creates a new TransactionDb for Tests
func setupKvStoreTest(database Databaser) KeyValueStore {
	return NewKeyValueStore(database, "test")
}

// TestAddKeyToTransaction Check that a single value is stored after made commit
func TestKeyValueStoreNewDbAndCommit(t *testing.T) {
	dbKV := NewKeyValueDb(t.TempDir(), 0)
	database := setupKvStoreTest(dbKV)
	database.Begin()

	database.Put([]byte("key"), []byte("value"))

	get, has := database.Get([]byte("key"))
	if !has || get == nil {
		t.Fail()
	}

	if string(get) != "value" {
		t.Fail()
	}

	database.Delete([]byte("key"))

	get, has = database.Get([]byte("key"))
	if has || get != nil {
		t.Fail()
	}
	database.Rollback()

	database.Close()
}
