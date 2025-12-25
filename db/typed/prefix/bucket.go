package prefix

import (
	"iter"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/NethermindEth/juno/utils"
)

type Entry[V any] struct {
	Key   []byte
	Value V
}

type scanner[B any] interface {
	scan(database db.KeyValueReader, prefix []byte) iter.Seq2[Entry[B], error]
}

type PrefixedBucket[T tag[V], K any, V any, KS key.Serializer[K], VS value.Serializer[V]] struct {
	typed.Bucket[K, V, KS, VS]
}

func NewPrefixedBucket[T tag[V], K any, V any, KS key.Serializer[K], VS value.Serializer[V]](
	bucket typed.Bucket[K, V, KS, VS],
	prefixFactory proxy[V, T],
) PrefixedBucket[T, K, V, KS, VS] {
	return PrefixedBucket[T, K, V, KS, VS]{
		Bucket: bucket,
	}
}

func (b *PrefixedBucket[T, K, V, KS, VS]) Prefix() T {
	return T{
		scanState: scanState[V]{
			prefix:  b.Bucket.Key(),
			scanner: b,
		},
	}
}

func (b *PrefixedBucket[T, K, V, KS, VS]) scan( //nolint:unused // False positive, used via scanner
	database db.KeyValueReader,
	prefix []byte,
) iter.Seq2[Entry[V], error] {
	return func(yield func(Entry[V], error) bool) {
		it, err := database.NewIterator(prefix, true)
		if err != nil {
			yield(Entry[V]{}, err)
			return
		}

		isExited, err := iterate[V, VS](it, yield)
		if err := utils.RunAndWrapOnError(it.Close, err); !isExited && err != nil {
			yield(Entry[V]{}, err)
		}
	}
}

func iterate[V any, VS value.Serializer[V]]( //nolint:unused // False positive, used in scan
	it db.Iterator,
	yield func(Entry[V], error) bool,
) (isExited bool, returnErr error) {
	for it.First(); it.Valid(); it.Next() {
		var entry Entry[V]
		entry.Key = it.Key()

		valueBytes, err := it.Value()
		if err != nil {
			return false, err
		}

		if err := (VS{}).Unmarshal(valueBytes, &entry.Value); err != nil {
			return false, err
		}

		if !yield(entry, nil) {
			return true, nil
		}
	}

	return false, nil
}
