package serialize

import "encoding/json"

func MarshalJson(in any) ([]byte, error) {
	return json.Marshal(in)
}

func UnMarshalJson[T any](b []byte) (T, error) {
	var t T
	err := json.Unmarshal(b, &t)

	return t, err
}
