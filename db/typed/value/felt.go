package value

type feltCodec interface {
	Marshal() []byte
	SetBytesCanonical([]byte) error
}

type feltBytesSerializer[F feltCodec] struct{}

func (feltBytesSerializer[F]) Marshal(value F) ([]byte, error) {
	return value.Marshal(), nil
}

func (feltBytesSerializer[F]) Unmarshal(data []byte, value F) error {
	return value.SetBytesCanonical(data)
}
