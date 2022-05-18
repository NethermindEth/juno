package db_test

import (
	"github.com/NethermindEth/juno/pkg/db"
	"testing"
)

// setupTransactionDbTest creates a new TransactionDb for Tests
func setupKvStoreTest(database db.Databaser) db.KeyValueStore {
	return db.NewKeyValueStore(database, "test")
}

// TestAddKeyToTransaction Check that a single value is stored after made commit
func TestKeyValueStoreNewDbAndCommit(t *testing.T) {
	dbKV := db.NewKeyValueDb(t.TempDir(), 0)
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
