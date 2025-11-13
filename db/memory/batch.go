package memory

import (
	"bytes"
	"maps"
	"slices"

	"github.com/NethermindEth/juno/db"
)

var _ db.Batch = (*batch)(nil)

type batch struct {
	db *Database
	// Theoretically, we can only maintain the latest write for each key using the map.
	// However, we want to ensure that the order of writes is maintained and mimics the
	// behaviour of the real key-value store. Hence, we store them and then flush them afterwards.
	writes   []keyValue
	writeMap map[string]keyValue
	size     int
}

type keyValue struct {
	key    string
	value  []byte
	delete bool
}

func newBatch(db *Database) *batch {
	return &batch{
		db:       db,
		writeMap: make(map[string]keyValue),
	}
}

func (b *batch) Get(key []byte, cb func(value []byte) error) error {
	b.db.lock.RLock()
	defer b.db.lock.RUnlock()

	if val, ok := b.writeMap[string(key)]; ok {
		if val.delete {
			return db.ErrKeyNotFound
		}
		return cb(val.value)
	}

	val, ok := b.db.db[string(key)]
	if !ok {
		return db.ErrKeyNotFound
	}

	return cb(val)
}

func (b *batch) Has(key []byte) (bool, error) {
	b.db.lock.RLock()
	defer b.db.lock.RUnlock()

	if val, ok := b.writeMap[string(key)]; ok {
		if val.delete {
			return false, nil
		}
		return true, nil
	}

	_, ok := b.db.db[string(key)]
	if ok {
		return true, nil
	}

	return ok, nil
}

func (b *batch) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	// create a temporary db
	tempDB := b.db.Copy()

	// copy the writes on this batch to the temporary db
	tempBatch := &batch{
		db:       tempDB,
		writes:   slices.Clone(b.writes),
		writeMap: maps.Clone(b.writeMap),
	}

	// write the changes to the temporary db
	if err := tempBatch.Write(); err != nil {
		return nil, err
	}

	// create a new iterator on the temporary db
	return tempDB.NewIterator(prefix, withUpperBound)
}

func (b *batch) Put(key, value []byte) error {
	kv := keyValue{key: string(key), value: slices.Clone(value)}
	b.writes = append(b.writes, kv)
	b.writeMap[string(key)] = kv
	b.size += len(key) + len(value)
	return nil
}

func (b *batch) Delete(key []byte) error {
	kv := keyValue{key: string(key), delete: true}
	b.writes = append(b.writes, kv)
	b.writeMap[string(key)] = kv
	b.size += len(key)
	return nil
}

func (b *batch) DeleteRange(start, end []byte) error {
	it, err := b.NewIterator(start, false)
	if err != nil {
		return err
	}

	for it.Next() {
		if bytes.Compare(it.Key(), end) >= 0 {
			break
		}

		if err := b.Delete(it.Key()); err != nil {
			return err
		}
	}

	return nil
}

func (b *batch) Size() int {
	return b.size
}

func (b *batch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	if b.db.db == nil {
		return errDBClosed
	}

	for _, write := range b.writes {
		if write.delete {
			delete(b.db.db, write.key)
		} else {
			b.db.db[write.key] = write.value
		}
	}

	b.Reset()
	return nil
}

func (b *batch) Reset() {
	b.size = 0
	b.writes = b.writes[:0] // reuse the memory
	b.writeMap = make(map[string]keyValue)
}

func (b *batch) Close() error {
	return nil
}
