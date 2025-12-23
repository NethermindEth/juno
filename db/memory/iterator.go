package memory

import (
	"bytes"
	"errors"
	"slices"

	"github.com/NethermindEth/juno/db"
)

var _ db.Iterator = (*iterator)(nil)

type iterator struct {
	curInd int
	keys   []string
	values [][]byte
	closed bool
}

func (i *iterator) Valid() bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	return i.curInd >= 0 && i.curInd < len(i.keys)
}

func (i *iterator) First() bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	i.curInd = 0
	return i.Valid()
}

func (i *iterator) Prev() bool {
	if i.closed {
		panic(errIteratorClosed)
	}

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
	if i.closed {
		panic(errIteratorClosed)
	}

	i.curInd++
	return i.Valid()
}

func (i *iterator) Key() []byte {
	if i.closed {
		panic(errIteratorClosed)
	}

	if !i.Valid() {
		return nil
	}

	return []byte(i.keys[i.curInd])
}

func (i *iterator) Value() ([]byte, error) {
	if i.closed {
		return nil, errIteratorClosed
	}

	bytes, err := i.UncopiedValue()
	if err != nil {
		return nil, err
	}
	return slices.Clone(bytes), nil
}

// DO NOT USE this if you don't unmarshal the value immediately.
// See [db.Iterator] for more details.
func (i *iterator) UncopiedValue() ([]byte, error) {
	if i.closed {
		return nil, errIteratorClosed
	}

	if !i.Valid() {
		return nil, errors.New("iterator is not valid")
	}

	return i.values[i.curInd], nil
}

func (i *iterator) Seek(key []byte) bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	for j := range i.keys {
		if bytes.Compare(key, []byte(i.keys[j])) <= 0 {
			i.curInd = j
			return true
		}
	}

	return false
}

func (i *iterator) Close() error {
	if i.closed {
		return errIteratorClosed
	}

	*i = iterator{
		curInd: -1,
		keys:   nil,
		values: nil,
		closed: true,
	}
	return nil
}
