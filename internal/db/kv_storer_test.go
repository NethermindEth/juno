package db

import (
	"testing"
)

// setupTransactionDbTest creates a new TransactionDb for Tests
func setupKvStoreTest(database Database) KeyValueStore {
	return NewKeyValueStore(database, "test")
}

// TestAddKeyToTransaction Check that a single value is stored after made commit
func TestKeyValueStoreNewDbAndCommit(t *testing.T) {
	env, err := NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	dbKV, err := NewMDBXDatabaseWithEnv(env, "KeyValueStore")
	if err != nil {
		t.Error(err)
	}
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

	dbKV.Close()
}
