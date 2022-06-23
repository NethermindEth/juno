package types

import (
	"encoding/json"
)

type PedersenHash Felt

func BytesToPedersenHash(b []byte) PedersenHash {
	return PedersenHash(BytesToFelt(b))
}

func HexToPedersenHash(s string) PedersenHash {
	return PedersenHash(HexToFelt(s))
}

func (t PedersenHash) Felt() Felt {
	return Felt(t)
}

func (t PedersenHash) Bytes() []byte {
	return t.Felt().Bytes()
}

func (t PedersenHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(Felt(t))
}

func (t PedersenHash) String() string {
	return Felt(t).String()
}

func (t *PedersenHash) UnmarshalJSON(data []byte) error {
	f := Felt{}
	err := f.UnmarshalJSON(data)
	if err != nil {
		return err
	}
	*t = PedersenHash(f)
	return nil
}
