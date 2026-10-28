package value

type feltCodec interface {
	StoredBytes() []byte
	SetMinimalBytes([]byte) error
}

// feltBytesSerializer writes a felt as felt.StoredBytes does, full width or trimmed by build,
// and reads either width.
type feltBytesSerializer[F feltCodec] struct{}

func (feltBytesSerializer[F]) Marshal(value F) ([]byte, error) {
	return value.StoredBytes(), nil
}

func (feltBytesSerializer[F]) Unmarshal(data []byte, value F) error {
	return value.SetMinimalBytes(data)
}
