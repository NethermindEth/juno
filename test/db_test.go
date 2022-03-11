package test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/pkg/db"
)

var keyValueTest = map[string]string{}

func init() {
	for i := 0; i < 350; i++ {
		val := strconv.Itoa(i)
		keyValueTest["key"+val] = "value" + val
	}
}

// setupDatabaseForTest creates a new KVDatabase for Tests
func setupDatabaseForTest(path string) *db.KeyValueDb {
	return db.New(path, 0)
}

// TestAddKey Check that a single value is inserted without error
func TestAddKey(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	err := database.Put([]byte("key"), []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

// TestNumberOfItems Checks that in every moment the collection contains the right amount of items
func TestNumberOfItems(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	n, err := database.Count()
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if n != 0 {
		t.Log(err)
		t.Fail()
		return
	}
	for k, v := range keyValueTest {
		err := database.Put([]byte(k), []byte(v))
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}
	}
	n, err = database.Count()
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if int(n) != len(keyValueTest) {
		t.Log(err)
		t.Fail()
	}
}

// TestAddMultipleKeys Checks that after insert some keys the collection contains the right amount of items
func TestAddMultipleKeys(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	for k, v := range keyValueTest {
		err := database.Put([]byte(k), []byte(v))
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}
	}
	n, err := database.Count()
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if int(n) != len(keyValueTest) {
		t.Log(err)
		t.Fail()
	}
}

// TestHasKey Check that one key exist after insertion
func TestHasKey(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	goodKey := []byte("good_key")
	err := database.Put(goodKey, []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	has, err := database.Has(goodKey)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if !has {
		t.Log(err)
		t.Fail()
		return
	}
}

// TestHasNotKey Check that a key don't exist
func TestHasNotKey(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	goodKey := []byte("good_key")
	badKey := []byte("bad_key")
	err := database.Put(goodKey, []byte("value"))
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	has2, err := database.Has(badKey)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if has2 {
		t.Log(err)
		t.Fail()
	}
}

// TestGetKey Check that a key is property retrieved
func TestGetKey(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	goodKey := []byte("good_key")
	goodValue := []byte("value")
	err := database.Put(goodKey, goodValue)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	val, err := database.Get(goodKey)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	if string(val) != string(goodValue) {
		t.Log(err)
		t.Fail()
	}
}

// TestGetNotKey Check that a key don't exist and what happen if it doesn't exist
func TestGetNotKey(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	goodKey := []byte("good_key")
	goodValue := []byte("value")
	badKey := []byte("bad_key")
	err := database.Put(goodKey, goodValue)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	key, err := database.Get(badKey)
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

func TestDelete(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	goodKey := []byte("good_key")
	goodValue := []byte("value")
	err := database.Put(goodKey, goodValue)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	err = database.Delete(goodKey)
	if err != nil {
		t.Log(err)
		t.Fail()
		return
	}
	key, err := database.Has(goodKey)
	if key {
		t.Log(err)
		t.Fail()
		return
	}
	err = database.Delete(goodKey)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
}

func TestBegin(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	database.Begin()
}
func TestRollBack(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	database.Rollback()
}
func TestClose(t *testing.T) {
	database := setupDatabaseForTest(t.TempDir())
	database.Close()
}

// BenchmarkEntriesInDatabase Benchmark the entry of key-value pairs to the db
func BenchmarkEntriesInDatabase(b *testing.B) {
	database := setupDatabaseForTest(b.TempDir())
	for i := 0; i < b.N; i++ {
		val := []byte(strconv.Itoa(i))
		err := database.Put(val, val)
		if err != nil {
			b.Error("Benchmarking fails, error storing values")
			return
		}
	}
	n, err := database.Count()
	if err != nil {
		b.Errorf("Benchmarking fails, error getting the number of items: %s\n", err)
		b.Fail()
		return
	}
	if int(n) != b.N {
		b.Error("Benchmarking fails, mismatch between number of items to insert and the number inside db")
		b.Fail()
		return
	}

}

// BenchmarkConsultsToDatabase Benchmark the consult to a db
func BenchmarkConsultsToDatabase(b *testing.B) {
	database := setupDatabaseForTest(b.TempDir())
	for i := 0; i < b.N; i++ {
		val := []byte(strconv.Itoa(i))
		err := database.Put(val, val)
		if err != nil {
			b.Error("Benchmarking fails, error storing values")
			b.Fail()
			return
		}
		get, err := database.Get(val)
		if err != nil {
			b.Errorf("Benchmarking fails, error getting values: %s\n", err)
			b.Fail()
			return
		}
		if string(get) != string(val) {
			b.Error("Benchmarking fails, mismatch between expected return value and returned value")
			b.Fail()
		}
	}
}
