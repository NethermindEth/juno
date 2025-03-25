package db

type Mapping[K any, V any] interface {
	~struct{}
	EncodeKey(K) ([]byte, error)
	EncodeValue(V) ([]byte, error)
	DecodeValue([]byte) (V, error)
}

type KeyValueStoreTyped[M Mapping[K, V], K any, V any] struct {
	KeyValueStore
}

func Typed[M Mapping[K, V], K any, V any](store KeyValueStore) KeyValueStoreTyped[M, K, V] {
	return KeyValueStoreTyped[M, K, V]{KeyValueStore: store}
}

func (store KeyValueStoreTyped[M, K, V]) Get(key K) (V, error) {
	var mapping M
	var res V

	keyByte, err := mapping.EncodeKey(key)
	if err != nil {
		return res, err
	}

	err = store.KeyValueStore.Get(keyByte, func(data []byte) error {
		res, err = mapping.DecodeValue(data)
		return err
	})

	return res, err
}

func (store KeyValueStoreTyped[M, K, V]) Put(key K, value V) error {
	var mapping M

	keyByte, err := mapping.EncodeKey(key)
	if err != nil {
		return err
	}

	valueByte, err := mapping.EncodeValue(value)
	if err != nil {
		return err
	}

	return store.KeyValueStore.Put(keyByte, valueByte)
}
