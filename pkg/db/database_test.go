package db

import "testing"

var (
	KeyValueTest = map[string]string{
		"key0": "value0",
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
		"key6": "value6",
		"key7": "value7",
		"key8": "value8",
		"key9": "value9",
	}
)

func setupTest(t *testing.T) KeyValueDatabase {
	path := t.TempDir()
	return NewKeyValueDatabase(path, 0)
}

func TestAddKey(t *testing.T) {
	db := setupTest(t)
	err := db.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func TestNumberOfItems(t *testing.T) {
	db := setupTest(t)
	n, err := db.NumberOfItems()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if n != 0 {
		t.Log(err)
		t.Fail()
	}
	for k, v := range KeyValueTest {
		err := db.Put([]byte(k), []byte(v))
		if err != nil {
			t.Log(err)
			t.Fail()
		}
	}
	n, err = db.NumberOfItems()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if int(n) != len(KeyValueTest) {
		t.Log(err)
		t.Fail()
	}
}

func TestAddMultipleKeys(t *testing.T) {
	db := setupTest(t)
	for k, v := range KeyValueTest {
		err := db.Put([]byte(k), []byte(v))
		if err != nil {
			t.Log(err)
			t.Fail()
		}
	}
	n, err := db.NumberOfItems()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if int(n) != len(KeyValueTest) {
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

func TestHasNotKey(t *testing.T) {
	db := setupTest(t)
	goodKey := []byte("good_key")
	badKey := []byte("bad_key")
	err := db.Put(goodKey, []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	has2, err := db.Has(badKey)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if has2 {
		t.Log(err)
		t.Fail()
	}
}

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
	key, err := db.Get(badKey)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if key != nil {
		t.Log(err)
		t.Fail()
		return
	}
}
