package lazy

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

type Slice[T any] struct {
	indexes []int
	end     int
	data    []byte
}

func NewSlice[T any](indexes []int, end int, data []byte) Slice[T] {
	return Slice[T]{
		indexes: indexes,
		end:     end,
		data:    data,
	}
}

func (l Slice[T]) Get(index int) (T, error) {
	var value T

	if index < 0 || index >= len(l.indexes) {
		return value, db.ErrKeyNotFound
	}

	start := l.indexes[index]
	end := l.end
	if index < len(l.indexes)-1 {
		end = l.indexes[index+1]
	}

	err := encoder.Unmarshal(l.data[start:end], &value)
	return value, err
}

func (l Slice[T]) All() ([]T, error) {
	var err error
	items := make([]T, len(l.indexes))
	for i := range l.indexes {
		if items[i], err = l.Get(i); err != nil {
			return nil, err
		}
	}
	return items, nil
}
