package db_test

//
//import (
//	"github.com/NethermindEth/juno/pkg/db"
//	"testing"
//)
//
//// setupTransactionDbTest creates a new TransactionDb for Tests
//func setupTransactionDbTest(path string) *db.TransactionDb {
//	return db.NewTransactionDb(path, 0)
//}
//
//// TestAddKeyToTransaction Check that a single value is stored after made commit
//func TestInsertKeyOnTransactionDbAndCommit(t *testing.T) {
//	database := setupTransactionDbTest(t.TempDir())
//
//	database.Begin()
//
//	err := database.Put([]byte("key"), []byte("value"))
//
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//
//	err = database.Commit()
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//
//	database.Begin()
//
//	get, err := database.Get([]byte("key"))
//	if err != nil || get == nil {
//		t.Log(err)
//		t.Fail()
//	}
//
//	if string(get) != "value" {
//		t.Fail()
//	}
//
//	database.Close()
//}
//
//// TestInsertKeyOnTransactionDbAndRollback Check that a single is deleted after a rollback
//func TestInsertKeyOnTransactionDbAndRollback(t *testing.T) {
//	database := setupTransactionDbTest(t.TempDir())
//
//	numItems, _ := database.NumberOfItems()
//	if numItems != 0 {
//		t.Fail()
//	}
//
//	database.Begin()
//
//	err := database.Put([]byte("key"), []byte("value"))
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//
//	numItems, _ = database.NumberOfItems()
//	if numItems != 1 {
//		t.Fail()
//	}
//
//	database.Rollback()
//
//	database.Begin()
//
//	has, err := database.Has([]byte("key"))
//	if err != nil || has {
//		t.Log(err)
//		t.Fail()
//	}
//
//	numItems, _ = database.NumberOfItems()
//	if numItems != 0 {
//		t.Fail()
//	}
//
//	database.Close()
//}
//
//// TestDeletionOnTransactionDb Check that a key is inserted and deleted properly
//func TestDeletionOnTransactionDb(t *testing.T) {
//	database := setupTransactionDbTest(t.TempDir())
//
//	database.Begin()
//
//	err := database.Put([]byte("key"), []byte("value"))
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//
//	err = database.Commit()
//	if err != nil {
//		t.Fail()
//		return
//	}
//
//	database.Begin()
//
//	err = database.Delete([]byte("key"))
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//
//	has, err := database.Has([]byte("key"))
//	if err != nil || has {
//		t.Log(err)
//		t.Fail()
//	}
//
//	database.Rollback()
//
//	database.Begin()
//	has, err = database.Has([]byte("key"))
//	if err != nil || !has {
//		t.Log(err)
//		t.Fail()
//	}
//
//	database.Close()
//}
