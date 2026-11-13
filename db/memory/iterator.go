package memory

import (
	"errors"
	"slices"

	"github.com/NethermindEth/juno/db"
)

var _ db.Iterator = (*iterator)(nil)

type iterator struct {
	// curInd is clamped to [-1, len(keys)] so stepping past either end stays exhausted.
	curInd     int
	positioned bool
	keys       []string
	values     [][]byte
	closed     bool
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

	i.positioned = true
	i.curInd = 0
	return i.Valid()
}

// Prev on an unpositioned iterator moves to the first key, consistent with pebble.
func (i *iterator) Prev() bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	if !i.positioned {
		return i.First()
	}

	if i.curInd >= 0 {
		i.curInd--
	}
	return i.Valid()
}

func (i *iterator) Next() bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	if !i.positioned {
		return i.First()
	}

	if i.curInd < len(i.keys) {
		i.curInd++
	}
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

func (i *iterator) Last() bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	i.positioned = true
	i.curInd = len(i.keys) - 1
	return i.Valid()
}

// SeekGE positions one past the end when key is past all entries, so that Prev() gives the
// last valid key, consistent with pebble's behaviour.
func (i *iterator) SeekGE(key []byte) bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	i.positioned = true
	i.curInd, _ = slices.BinarySearch(i.keys, string(key))
	return i.Valid()
}

func (i *iterator) SeekLT(key []byte) bool {
	if i.closed {
		panic(errIteratorClosed)
	}

	i.positioned = true
	index, _ := slices.BinarySearch(i.keys, string(key))
	i.curInd = index - 1
	return i.Valid()
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
