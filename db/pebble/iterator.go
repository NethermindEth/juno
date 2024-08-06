package pebble

import (
	"slices"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

var _ db.Iterator = (*iterator)(nil)

type iterator struct {
	iter       *pebble.Iterator
	positioned bool
}

// Valid : see db.Transaction.Iterator.Valid
func (i *iterator) Valid() bool {
	return i.iter.Valid()
}

// Key : see db.Transaction.Iterator.Key
func (i *iterator) Key() []byte {
	key := i.iter.Key()
	if key == nil {
		return nil
	}
	buf := slices.Clone(key)
	return buf
}

// Value : see db.Transaction.Iterator.Value
func (i *iterator) Value() ([]byte, error) {
	val, err := i.iter.ValueAndErr()
	if err != nil || val == nil {
		return nil, err
	}
	buf := slices.Clone(val)
	return buf, nil
}

// Next : see db.Transaction.Iterator.Next
func (i *iterator) Next() bool {
	if !i.positioned {
		i.positioned = true
		return i.iter.First()
	}
	return i.iter.Next()
}

// Seek : see db.Transaction.Iterator.Seek
func (i *iterator) Seek(key []byte) bool {
	i.positioned = true
	return i.iter.SeekGE(key)
}

// Close : see db.Transaction.Iterator.Close
func (i *iterator) Close() error {
	return i.iter.Close()
}
