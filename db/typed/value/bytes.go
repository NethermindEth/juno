package value

import "slices"

type BytesSerializer struct{}

func (BytesSerializer) Marshal(value *[]byte) ([]byte, error) {
	return *value, nil
}

func (BytesSerializer) Unmarshal(data []byte, value *[]byte) error {
	*value = slices.Clone(data)
	return nil
}
