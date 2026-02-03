package partial

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
)

type PartialSerializer[K, V any] interface {
	~struct{}
	UnmarshalPartial(subKey K, data []byte, value *V) error
}

type PartialBucket[K, K2, V any, KS key.Serializer[K], VS PartialSerializer[K2, V]] struct {
	db.Bucket
}

func NewPartialBucket[
	K, K2, V, V2 any,
	KS key.Serializer[K],
	VS value.Serializer[V],
	V2S PartialSerializer[K2, V2],
](
	bucket typed.Bucket[K, V, KS, VS],
	valueSerializer V2S,
) PartialBucket[K, K2, V2, KS, V2S] {
	return PartialBucket[K, K2, V2, KS, V2S]{
		Bucket: bucket.Bucket,
	}
}

func (b PartialBucket[K, K2, V, KS, VS]) Get(
	database db.KeyValueReader,
	key K,
	subKey K2,
) (V, error) {
	var value V
	err := database.Get(b.Key(KS{}.Marshal(key)), func(data []byte) error {
		return VS{}.UnmarshalPartial(subKey, data, &value)
	})
	return value, err
}
