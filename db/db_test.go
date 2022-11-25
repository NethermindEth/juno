package db

import (
	"testing"
)

func testDb(t *testing.T, db Database) {
	bucketName := "testBucket"
	kvs := []struct {
		k, v []byte
	}{
		{[]byte("testKey1"), []byte("testValue1")},
		{[]byte("testKey2"), []byte("testValue2")},
		{[]byte("testKey31"), []byte("testValue31")}, // Add trailing '1' to test cursor.Seek() later
	}

	// View
	err := db.View(func(tx Transaction) error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed view transaction: %s", err)
	}

	// View: try to modify the database
	err = db.View(func(tx Transaction) error {
		if err := tx.CreateBucketIfNotExists(bucketName); err == nil {
			t.Fatalf("db permitted view transaction to modify db")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error in db.View: %s", err)
	}

	// Update
	err = db.Update(func(_ Transaction) error {
		return nil
	})
	if err != nil {
		t.Fatalf("failed to update db: %s", err)
	}

	// Update: try to modify the database
	err = db.Update(func(tx Transaction) error {
		// Transaction: create bucket
		if err := tx.CreateBucketIfNotExists(bucketName); err != nil {
			t.Fatalf("failed to create bucket: %s", err)
		}

		// Transaction: retrieve cursor for nonexistant bucket
		_, err := tx.Cursor("fakeBucket")
		if err == nil {
			t.Fatalf("transaction obtained cursor for nonexistant bucket")
		}

		// Transaction: obtain cursor for existing bucket
		c, err := tx.Cursor(bucketName)
		if err != nil {
			t.Fatalf("transaction failed to retrieve existing bucket \"%s\": %s", bucketName, err)
		}

		// Cursor: insert values
		for _, kv := range kvs {
			if err = c.Put(kv.k, kv.v); err != nil {
				t.Fatalf("cursor failed to insert (k: \"%s\", v: \"%s\"): %s", kv.k, kv.v, err)
			}
		}

		// Cursor: get a value
		if v, err := c.Get(kvs[0].k); err != nil {
			t.Fatalf("cursor failed to get value for key \"%s\": %s", kvs[0].k, err)
		} else if string(v) != string(kvs[0].v) {
			t.Fatalf("cursor wrong value for key \"%s\": got \"%s\", want \"%s\"", kvs[0].k, v, kvs[0].v)
		}

		// Cursor: last, first
		lastKv := kvs[len(kvs)-1]
		if k, v, err := c.Last(); err != nil {
			t.Fatalf("cursor.Last() returned unexpected error: %s", err)
		} else if string(k) != string(lastKv.k) || string(v) != string(lastKv.v) {
			t.Fatalf("cursor.Last() returned incorrect pair: got (\"%s\", \"%s\"), want (\"%s\", \"%s\")", k, v, lastKv.k, lastKv.v)
		}
		if k, v, err := c.First(); err != nil {
			t.Fatalf("cursor.First() returned unexpected error: \"%s\"", err)
		} else if string(k) != string(kvs[0].k) || string(v) != string(kvs[0].v) {
			t.Fatalf("cursor.First() returned incorrect pair: got (\"%s\", \"%s\"), want (\"%s\", \"%s\")", k, v, kvs[0].k, kvs[0].v)
		}

		// Cursor: next, prev
		if k, v, err := c.Next(); err != nil {
			t.Fatalf("cursor.Next() returned unexpected error")
		} else if string(k) != string(kvs[1].k) || string(v) != string(kvs[1].v) {
			t.Fatalf("cursor.Next() returned incorrect pair: got (\"%s\", \"%s\"), want (\"%s\", \"%s\")", k, v, kvs[1].k, kvs[1].v)
		}
		if k, v, err := c.Prev(); err != nil {
			t.Fatalf("cursor.Prev() returned unexpected error: %s", err)
		} else if string(k) != string(kvs[0].k) || string(v) != string(kvs[0].v) {
			t.Fatalf("cursor.Prev() returned incorrect pair: got (\"%s\", \"%s\"), want (\"%s\", \"%s\")", k, v, kvs[0].k, kvs[0].v)
		}

		// Cursor: seek
		target := kvs[2]
		prefix := target.k[:len(target.k)-1] // All but the last character
		if k, v, err := c.Seek(prefix); err != nil {
			t.Fatalf("cursor.Seek(\"%s\") returned unexpected error: %s", prefix, err)
		} else if string(k) != string(target.k) || string(v) != string(target.v) {
			t.Fatalf("cursor.Seek(\"%s\") returned incorrect pair: got (\"%s\", \"%s\"), want (\"%s\", \"%s\")", prefix, k, v, target.k, target.v)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to update db: %s", err)
	}

	// Run multiple transactions

	// View
	err = db.View(func(tx Transaction) error {
		c, err := tx.Cursor(bucketName)
		if err != nil {
			t.Fatalf("failed to open cursor on existing bucket %s: %s", bucketName, err)
		}
		kv := kvs[0]
		if k, v, err := c.First(); err != nil {
			t.Fatalf("unexpected error on cursor.First(): %s", err)
		} else if string(k) != string(kv.k) || string(v) != string(kv.v) {
			t.Fatalf("cursor.First() did not return correct key-value pair: got (\"%s\", \"%s\"), want (\"%s\", \"%s\")", k, v, kv.k, kv.v)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to view db: %s", err)
	}

	// Update
	err = db.Update(func(tx Transaction) error {
		c, err := tx.Cursor(bucketName)
		if err != nil {
			t.Fatalf("failed to open cursor on existing bucket %s: %s", bucketName, err)
		}
		if err = c.Put([]byte("testKey4"), []byte("testValue4")); err != nil {
			t.Fatalf("unexpected error on cursor.Put(\"testKey4\", \"testValue4\"): %s", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to update db: %s", err)
	}
}

func TestMdbxDb(t *testing.T) {
	db, err := NewMdbxDb(t.TempDir(), 1 /* maxBuckets */)
	if err != nil {
		t.Fatalf("failed to create db: %s", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close db: %s", err)
		}
	}()

	testDb(t, db)
}
