package value

type BytesSerializer struct{}

func (BytesSerializer) Marshal(value *[]byte) ([]byte, error) {
	return *value, nil
}

func (BytesSerializer) Unmarshal(data []byte, value *[]byte) error {
	*value = data
	return nil
}
