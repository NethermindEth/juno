package typed

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
)

type Bucket[K any, V any, KS key.Serializer[K], VS value.Serializer[V]] struct {
	db.Bucket
}

func NewBucket[K any, V any, KS key.Serializer[K], VS value.Serializer[V]](
	bucket db.Bucket,
	keySerializer KS,
	valueSerializer VS,
) Bucket[K, V, KS, VS] {
	return Bucket[K, V, KS, VS]{
		Bucket: bucket,
	}
}

func (b Bucket[K, V, KS, VS]) RawKey() Bucket[[]byte, V, key.BytesSerializer, VS] {
	return Bucket[[]byte, V, key.BytesSerializer, VS](b)
}

func (b Bucket[K, V, KS, VS]) RawValue() Bucket[K, []byte, KS, value.BytesSerializer] {
	return Bucket[K, []byte, KS, value.BytesSerializer](b)
}

func (b Bucket[K, V, KS, VS]) Has(database db.KeyValueReader, key K) (bool, error) {
	return database.Has(b.Key(KS{}.Marshal(key)))
}

func (b Bucket[K, V, KS, VS]) Get(database db.KeyValueReader, key K) (V, error) {
	var value V
	err := database.Get(b.Key(KS{}.Marshal(key)), func(data []byte) error {
		return VS{}.Unmarshal(data, &value)
	})
	return value, err
}

func (b Bucket[K, V, KS, VS]) Put(database db.KeyValueWriter, key K, value *V) error {
	data, err := VS{}.Marshal(value)
	if err != nil {
		return err
	}
	return database.Put(b.Key(KS{}.Marshal(key)), data)
}

func (b Bucket[K, V, KS, VS]) Delete(database db.KeyValueWriter, key K) error {
	return database.Delete(b.Key(KS{}.Marshal(key)))
}
