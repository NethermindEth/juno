package key

type uint8Serializer struct{}

func (uint8Serializer) Marshal(value uint8) []byte {
	data := [1]byte{value}
	return data[:]
}
