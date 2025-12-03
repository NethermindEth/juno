package key

type BytesSerializer struct{}

func (BytesSerializer) Marshal(value []byte) []byte {
	return value
}
