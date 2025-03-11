package memory

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var _ db.Batch = (*batch)(nil)

type batch struct {
	db     *Database
	writes []keyValue
	size   int
}

type keyValue struct {
	key    string
	value  []byte
	delete bool
}

func (b *batch) Put(key, value []byte) error {
	b.writes = append(b.writes, keyValue{key: string(key), value: utils.CopySlice(value)})
	b.size += len(key) + len(value)
	return nil
}

func (b *batch) Delete(key []byte) error {
	b.writes = append(b.writes, keyValue{key: string(key), delete: true})
	b.size += len(key)
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
}
