package db

import "testing"

func setupTest(t *testing.T) Database {
	path := t.TempDir()
	return NewDatabase(path, 0)
}

func TestAddKey(t *testing.T) {
	db := setupTest(t)
	err := db.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func TestHasKey(t *testing.T) {

}

func TestDeleteKey(t *testing.T) {

}
