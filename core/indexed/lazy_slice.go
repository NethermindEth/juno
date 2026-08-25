package indexed

import (
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/cbor"
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

func (l LazySlice[T]) getInto(index int, value *T) error {
	start := l.indexes[index]
	end := len(l.data)
	if index < len(l.indexes)-1 {
		end = l.indexes[index+1]
	}
	return cbor.Unmarshal(l.data[start:end], value)
}

func (l LazySlice[T]) Get(index int) (T, error) {
	if index < 0 || index >= len(l.indexes) {
		return *new(T), db.ErrKeyNotFound
	}
	var value T
	err := l.getInto(index, &value)
	return value, err
}

func (l LazySlice[T]) All() ([]T, error) {
	items := make([]T, len(l.indexes))
	for i := range l.indexes {
		if err := l.getInto(i, &items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func AllMapped[T, R any](
	l LazySlice[T],
	extract func(index int, value T) (R, error),
) ([]R, error) {
	results := make([]R, len(l.indexes))
	var value T
	for i := range l.indexes {
		// Reset: decoding merges into the existing value, so stale data would leak across elements.
		value = *new(T)
		if err := l.getInto(i, &value); err != nil {
			return nil, fmt.Errorf("decoding element %d: %w", i, err)
		}
		var err error
		if results[i], err = extract(i, value); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (l LazySlice[T]) Iter() iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var value T
		for i := range l.indexes {
			value = *new(T)
			err := l.getInto(i, &value)
			if !yield(value, err) {
				return
			}
		}
	}
}
