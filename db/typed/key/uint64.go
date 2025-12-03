package key

import "encoding/binary"

type uint64Serializer struct{}

func (uint64Serializer) Marshal(value uint64) []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, value)
	return data
}
