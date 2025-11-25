package key

type bytesSerializer struct{}

func (bytesSerializer) Marshal(value []byte) []byte {
	return value
}
