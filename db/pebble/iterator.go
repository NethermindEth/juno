package pebble

import "github.com/cockroachdb/pebble"

type iterator struct {
	iter *pebble.Iterator
}

// Valid : see db.Transaction.Iterator.Valid
func (i *iterator) Valid() bool {
	return i.iter.Valid()
}

// Key : see db.Transaction.Iterator.Key
func (i *iterator) Key() []byte {
	return i.iter.Key()
}

// Value : see db.Transaction.Iterator.Value
func (i *iterator) Value() ([]byte, error) {
	return i.iter.ValueAndErr()
}

// Next : see db.Transaction.Iterator.Next
func (i *iterator) Next() bool {
	return i.iter.Next()
}

// Seek : see db.Transaction.Iterator.Seek
func (i *iterator) Seek(key []byte) bool {
	return i.iter.SeekGE(key)
}

// Close : see db.Transaction.Iterator.Close
func (i *iterator) Close() error {
	return i.iter.Close()
}
