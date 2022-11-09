package db

import (
	"os"
	"path/filepath"
	"testing"
)

func testDb(t *testing.T, db Database) {
	err := db.View(func(_ Transaction) error {
		return nil
	})
	if err != nil {
		t.Errorf("failed to view db: %s", err)
	}
	err = db.Update(func(_ Transaction) error {
		return nil
	})
	if err != nil {
		t.Errorf("failed to update db: %s", err)
	}
}

func testTx(t *testing.T, tx Transaction) Bucket {
	bucketName := []byte("test")
	b, err := tx.CreateBucket(bucketName)
	if err != nil {
		t.Errorf("failed to create bucket: %s", err)
	}
	if _, err = tx.Bucket(bucketName); err != nil {
		t.Errorf("failed to retrieve bucket: %s", err)
	}
	return b
}

func testBucket(t *testing.T, b Bucket) {
	key := []byte("k")
	want := []byte("v")
	if err := b.Put(key, want); err != nil {
		t.Errorf("failed to insert: %s", err)
	}
	got, err := b.Get(key)
	if err != nil {
		t.Errorf("failed to retrieve: %s", err)
	}
	if string(got) != string(want) {
		t.Errorf("incorrect value: got \"%s\", want \"%s\"", got, want)
	}
}

// Only tests default bolt options.
func TestDefaultBoltDb(t *testing.T) {
	db, err := NewBoltDb(filepath.Join(os.TempDir(), "test.db"), 0600, nil)
	if err != nil {
		t.Errorf("failed to create db: %s", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close db: %s", err)
		}
	}()

	boltTx, err := db.db.Begin(true)
	if err != nil {
		t.Errorf("failed to create tx: %s", err)
	}
	tx := newBoltTx(boltTx)

	b := testTx(t, tx)
	testBucket(t, b)

	// Rollback transaction before testing the db
	// Otherwise db causes a panic by trying to obtain an exclusive lock on the file
	if err := boltTx.Rollback(); err != nil {
		t.Errorf("failed to rollback tx: %s", err)
	}

	testDb(t, db)
}
