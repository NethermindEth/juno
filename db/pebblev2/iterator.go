package pebblev2

import (
	"slices"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
)

var _ db.Iterator = (*iterator)(nil)

type iterator struct {
	iter       *pebble.Iterator
	positioned bool
}

func (i *iterator) Valid() bool {
	return i.iter.Valid()
}

func (i *iterator) Key() []byte {
	key := i.iter.Key()
	if key == nil {
		return nil
	}
	buf := slices.Clone(key)
	return buf
}

func (i *iterator) Value() ([]byte, error) {
	val, err := i.iter.ValueAndErr()
	if err != nil || val == nil {
		return nil, err
	}
	buf := slices.Clone(val)
	return buf, nil
}

func (i *iterator) First() bool {
	i.positioned = true
	return i.iter.First()
}

func (i *iterator) Prev() bool {
	if !i.positioned {
		i.positioned = true
		return i.iter.First()
	}
	return i.iter.Prev()
}

func (i *iterator) Next() bool {
	if !i.positioned {
		i.positioned = true
		return i.iter.First()
	}
	return i.iter.Next()
}

func (i *iterator) Seek(key []byte) bool {
	i.positioned = true
	return i.iter.SeekGE(key)
}

func (i *iterator) Close() error {
	return i.iter.Close()
}
