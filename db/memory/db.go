package memory

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/utils"
)

var (
	errDBClosed    = errors.New("memory database closed")
	errKeyNotFound = errors.New("key not found")
)

var _ db.KeyValueStore = (*Database)(nil)

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

func (d *Database) Get2(key []byte) ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if d.db == nil {
		return nil, errDBClosed
	}

	value, ok := d.db[string(key)]
	if !ok {
		return nil, errKeyNotFound
	}

	return utils.CopySlice(value), nil
}

func (d *Database) Put(key, value []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.db == nil {
		return errDBClosed
	}

	d.db[string(key)] = utils.CopySlice(value)
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
	}, nil
}

func (d *Database) NewSnapshot() db.Snapshot {
	if d.db == nil {
		panic(errDBClosed)
	}
	return d.copy()
}

func (d *Database) Update2(fn func(db.KeyValueWriter) error) error {
	if d.db == nil {
		return errDBClosed
	}

	batch := d.NewBatch()
	if err := fn(batch); err != nil {
		return err
	}

	d.lock.Lock()
	defer d.lock.Unlock()
	return batch.Write()
}

func (d *Database) View2(fn func(db.Snapshot) error) error {
	if d.db == nil {
		return errDBClosed
	}

	snap := d.NewSnapshot()
	return fn(snap)
}

func (d *Database) WithListener2(listener db.EventListener) db.KeyValueStore {
	return d // no-op
}

// Returns a deep copy of the key-value store
func (d *Database) copy() *Database {
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
