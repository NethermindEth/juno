package db

import "testing"

func setupTest(t *testing.T) KeyValueDatabase {
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
	db := setupTest(t)
	goodKey := []byte("good_key")
	err := db.Put(goodKey, []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	has, err := db.Has(goodKey)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if !has {
		t.Log(err)
		t.Fail()
	}
}

//func TestHasNotKey(t *testing.T) {
//	db := setupTest(t)
//	goodKey := []byte("good_key")
//	badKey := []byte("bad_key")
//	err := db.Put(goodKey, []byte("value"))
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//	has2, err := db.Has(badKey)
//	if err != nil {
//		t.Log(err)
//		t.Fail()
//	}
//	if has2 {
//		t.Log(err)
//		t.Fail()
//	}
//}

func TestGetKey(t *testing.T) {
	db := setupTest(t)
	goodKey := []byte("good_key")
	goodValue := []byte("value")
	err := db.Put(goodKey, goodValue)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	val, err := db.Get(goodKey)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if string(val) != string(goodValue) {
		t.Log(err)
		t.Fail()
	}
}

func TestGetNotKey(t *testing.T) {
	db := setupTest(t)
	goodKey := []byte("good_key")
	goodValue := []byte("value")
	badKey := []byte("bad_key")
	err := db.Put(goodKey, goodValue)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	_, err = db.Get(badKey)
	if err != nil {
		if err.Error() == "get: mdbx_get: MDBX_NOTFOUND: No matching key/data pair found" {
			return
		}
		t.Log(err)
		t.Fail()
		return
	}
}
