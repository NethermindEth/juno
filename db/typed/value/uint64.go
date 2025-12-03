package value

import "encoding/binary"

type uint64Serializer struct{}

func (uint64Serializer) Marshal(value *uint64) ([]byte, error) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, *value)
	return data, nil
}

func (uint64Serializer) Unmarshal(data []byte, value *uint64) error {
	*value = binary.BigEndian.Uint64(data)
	return nil
}
