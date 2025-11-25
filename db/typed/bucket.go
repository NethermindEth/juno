package typed

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
)

type Bucket[K any, V any, KS key.Serializer[K], VS value.Serializer[V]] struct {
	db.Bucket
	keySerializer   KS
	valueSerializer VS
}

func NewBucket[K any, V any, KS key.Serializer[K], VS value.Serializer[V]](
	bucket db.Bucket,
	keySerializer KS,
	valueSerializer VS,
) Bucket[K, V, KS, VS] {
	return Bucket[K, V, KS, VS]{
		Bucket:          bucket,
		keySerializer:   keySerializer,
		valueSerializer: valueSerializer,
	}
}

func (b Bucket[K, V, KS, VS]) RawKey() Bucket[[]byte, V, key.Serializer[[]byte], VS] {
	return Bucket[[]byte, V, key.Serializer[[]byte], VS]{
		Bucket:          b.Bucket,
		keySerializer:   key.Bytes,
		valueSerializer: b.valueSerializer,
	}
}

func (b Bucket[K, V, KS, VS]) RawValue() Bucket[K, []byte, KS, value.Serializer[[]byte]] {
	return Bucket[K, []byte, KS, value.Serializer[[]byte]]{
		Bucket:          b.Bucket,
		keySerializer:   b.keySerializer,
		valueSerializer: value.Bytes,
	}
}

func (b Bucket[K, V, KS, VS]) Has(database db.KeyValueReader, key K) (bool, error) {
	return database.Has(b.Key(b.keySerializer.Marshal(key)))
}

func (b Bucket[K, V, KS, VS]) Get(database db.KeyValueReader, key K) (V, error) {
	var value V
	err := database.Get(b.Key(b.keySerializer.Marshal(key)), func(data []byte) error {
		return b.valueSerializer.Unmarshal(data, &value)
	})
	return value, err
}

func (b Bucket[K, V, KS, VS]) Put(database db.KeyValueWriter, key K, value *V) error {
	data, err := b.valueSerializer.Marshal(value)
	if err != nil {
		return err
	}
	return database.Put(b.Key(b.keySerializer.Marshal(key)), data)
}

func (b Bucket[K, V, KS, VS]) Delete(database db.KeyValueWriter, key K) error {
	return database.Delete(b.Key(b.keySerializer.Marshal(key)))
}
