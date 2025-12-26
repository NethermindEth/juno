package indexed

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

// Items are stored contiguously in a data slice, with an indexes slice marking each
// item’s start offset. The data is encoded using CBOR and deserialized into type T
// on demand.
// Example:
//
// data:          [00|01|02|03|04|05|06|07|08|09|10|11|12|13|14|15|16|17|18|19]
// -                                             ----- -------- --------------
// -                                             ↑     ↑        ↑              ↑
// indexes:                                     [10    12       15]            20 (len(data))
type LazySlice[T any] struct {
	indexes []int
	data    []byte
}

func NewLazySlice[T any](indexes []int, data []byte) LazySlice[T] {
	return LazySlice[T]{
		indexes: indexes,
		data:    data,
	}
}

func (l LazySlice[T]) get(index int) (T, error) {
	start := l.indexes[index]
	end := len(l.data)
	if index < len(l.indexes)-1 {
		end = l.indexes[index+1]
	}

	var value T
	err := encoder.Unmarshal(l.data[start:end], &value)
	return value, err
}

func (l LazySlice[T]) Get(index int) (T, error) {
	if index < 0 || index >= len(l.indexes) {
		return *new(T), db.ErrKeyNotFound
	}
	return l.get(index)
}

func (l LazySlice[T]) All() ([]T, error) {
	var err error
	items := make([]T, len(l.indexes))
	for i := range l.indexes {
		if items[i], err = l.get(i); err != nil {
			return nil, err
		}
	}
	return items, nil
}
