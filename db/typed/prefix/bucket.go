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

type PrefixedBucket[T tag, K any, V any, KS key.Serializer[K], VS value.Serializer[V]] struct {
	typed.Bucket[K, V, KS, VS]
}

func NewPrefixedBucket[T tag, K any, V any, KS key.Serializer[K], VS value.Serializer[V]](
	bucket typed.Bucket[K, V, KS, VS],
	prefixFactory proxy[K, T],
) PrefixedBucket[T, K, V, KS, VS] {
	return PrefixedBucket[T, K, V, KS, VS]{
		Bucket: bucket,
	}
}

func (b *PrefixedBucket[T, K, V, KS, VS]) Prefix() T {
	return T{}
}

func (b *PrefixedBucket[T, K, V, KS, VS]) Scan(
	database db.Iterable,
	prefix end[K],
) iter.Seq2[Entry[V], error] {
	return func(yield func(Entry[V], error) bool) {
		it, err := database.NewIterator(b.Key(prefix), true)
		if err != nil {
			yield(Entry[V]{}, err)
			return
		}

		isError := false
		defer func() {
			if !isError {
				if err := it.Close(); err != nil {
					yield(Entry[V]{}, err)
				}
			}
		}()

		var vs VS
		for it.First(); it.Valid(); it.Next() {
			var entry Entry[V]
			entry.Key = it.Key()

			valueBytes, err := it.Value()
			if err != nil {
				yield(Entry[V]{}, utils.RunAndWrapOnError(it.Close, err))
				isError = true
				return
			}

			if err := vs.Unmarshal(valueBytes, &entry.Value); err != nil {
				yield(Entry[V]{}, utils.RunAndWrapOnError(it.Close, err))
				isError = true
				return
			}

			if !yield(entry, nil) {
				return
			}
		}
	}
}
