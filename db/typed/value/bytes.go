package value

type bytesSerializer struct{}

func (bytesSerializer) Marshal(value *[]byte) ([]byte, error) {
	return *value, nil
}

func (bytesSerializer) Unmarshal(data []byte, value *[]byte) error {
	*value = data
	return nil
}
