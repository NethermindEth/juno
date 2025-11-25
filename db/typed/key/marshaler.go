package key

type marshaler[K any] interface {
	Marshal() []byte
}

type marshalSerializer[K marshaler[K]] struct{}

func (marshalSerializer[K]) Marshal(value K) []byte {
	return value.Marshal()
}
