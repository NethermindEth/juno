package memory

import (
	"bytes"
	"errors"

	"github.com/NethermindEth/juno/db"
)

var _ db.Iterator = (*iterator)(nil)

type iterator struct {
	curInd int
	keys   []string
	values [][]byte
}

func (i *iterator) Valid() bool {
	return i.curInd >= 0 && i.curInd < len(i.keys)
}

func (i *iterator) First() bool {
	i.curInd = 0
	return i.Valid()
}

func (i *iterator) Prev() bool {
	if i.curInd == 0 {
		return false
	}

	if i.curInd == -1 {
		return i.First()
	}

	i.curInd--
	return true
}

func (i *iterator) Next() bool {
	i.curInd++
	return i.Valid()
}

func (i *iterator) Key() []byte {
	if !i.Valid() {
		return nil
	}

	return []byte(i.keys[i.curInd])
}

func (i *iterator) Value() ([]byte, error) {
	if !i.Valid() {
		return nil, errors.New("iterator is not valid")
	}

	return i.values[i.curInd], nil
}

func (i *iterator) Seek(key []byte) bool {
	for j := range i.keys {
		if bytes.Compare(key, []byte(i.keys[j])) <= 0 {
			i.curInd = j
			return true
		}
	}

	return false
}

func (i *iterator) Close() error {
	i.curInd = -1
	i.keys = nil
	i.values = nil
	return nil
}
