package utils

import "encoding/json"

// Base64 represents base64-encoded binary data serialised as a JSON string.
type Base64 string

func (b Base64) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(b))
}

func (b *Base64) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*b = Base64(s)
	return nil
}
