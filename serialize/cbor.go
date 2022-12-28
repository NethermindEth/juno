package serialize

import "github.com/fxamacker/cbor/v2"

func MarshalCbor(in any) ([]byte, error) {
	b, err := cbor.Marshal(in)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func UnMarshalCbor[T any](b []byte) (T, error) {
	var t T
	err := cbor.Unmarshal(b, &t)

	return t, err
}
