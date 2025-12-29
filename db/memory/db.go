package memory

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
)

var (
	errDBClosed       = errors.New("memory database closed")
	errBatchClosed    = errors.New("memory batch closed")
	errIteratorClosed = errors.New("memory iterator closed")
)

var _ db.KeyValueStore = (*Database)(nil)

// Represents an in-memory key-value store.
// It is thread-safe.
type Database struct {
	db   map[string][]byte
	lock sync.RWMutex
}

func New() *Database {
	return &Database{
		db: make(map[string][]byte),
	}
}

func (d *Database) Has(key []byte) (bool, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if d.db == nil {
		return false, errDBClosed
	}

	_, ok := d.db[string(key)]
	return ok, nil
}

func (d *Database) Get(key []byte, cb func(value []byte) error) error {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if d.db == nil {
		return errDBClosed
	}

	val, ok := d.db[string(key)]
	if !ok {
		return db.ErrKeyNotFound
	}

	return cb(val)
}

func (d *Database) Put(key, value []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.db == nil {
		return errDBClosed
	}

	d.db[string(key)] = slices.Clone(value)
	return nil
}

func (d *Database) Delete(key []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.db == nil {
		return errDBClosed
	}

	delete(d.db, string(key))
	return nil
}

func (d *Database) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.db = nil
	return nil
}

func (d *Database) DeleteRange(start, end []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.db == nil {
		return errDBClosed
	}

	for k := range d.db {
		// start is inclusive, end is exclusive
		if k >= string(start) && k < string(end) {
			delete(d.db, k)
		}
	}

	return nil
}

func (d *Database) NewBatch() db.Batch                            { return newBatch(d) }
func (d *Database) NewBatchWithSize(_ int) db.Batch               { return newBatch(d) }
func (d *Database) NewIndexedBatch() db.IndexedBatch              { return newBatch(d) }
func (d *Database) NewIndexedBatchWithSize(_ int) db.IndexedBatch { return newBatch(d) }

func (d *Database) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if d.db == nil {
		return nil, errDBClosed
	}

	var upperBound []byte
	if withUpperBound {
		upperBound = dbutils.UpperBound(prefix)
	}

	var (
		pr   = string(prefix)
		ub   = string(upperBound)
		keys = make([]string, 0, len(d.db))
		vals = make([][]byte, 0, len(d.db))
	)

	for k := range d.db {
		if strings.HasPrefix(k, pr) && (!withUpperBound || k < ub) {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)
	for _, k := range keys {
		vals = append(vals, d.db[k])
	}

	return &iterator{
		curInd: -1,
		keys:   keys,
		values: vals,
		closed: false,
	}, nil
}

func (d *Database) NewSnapshot() db.Snapshot {
	if d.db == nil {
		panic(errDBClosed)
	}
	return d.Copy()
}

func (d *Database) Update(fn func(db.IndexedBatch) error) error {
	if d.db == nil {
		return errDBClosed
	}

	batch := d.NewIndexedBatch()
	if err := fn(batch); err != nil {
		return err
	}

	return batch.Write()
}

func (d *Database) View(fn func(db.Snapshot) error) error {
	if d.db == nil {
		return errDBClosed
	}

	snap := d.NewSnapshot()
	defer snap.Close()
	return fn(snap)
}

func (d *Database) WithListener(listener db.EventListener) db.KeyValueStore {
	return d // no-op
}

// Returns a deep Copy of the key-value store
func (d *Database) Copy() *Database {
	d.lock.RLock()
	defer d.lock.RUnlock()

	cp := &Database{
		db: make(map[string][]byte, len(d.db)),
	}

	for k, v := range d.db {
		cp.db[k] = slices.Clone(v)
	}

	return cp
}

func (d *Database) Impl() any {
	return d.db
}
