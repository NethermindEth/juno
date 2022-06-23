package types

import (
	"encoding/json"
)

type ContractHash PedersenHash

func BytesToContractHash(b []byte) ContractHash {
	return ContractHash(BytesToPedersenHash(b))
}

func HexToContractHash(s string) ContractHash {
	return ContractHash(HexToPedersenHash(s))
}

func (t ContractHash) Felt() Felt {
	return PedersenHash(t).Felt()
}

func (t ContractHash) Bytes() []byte {
	return t.Felt().Bytes()
}

func (t ContractHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(Felt(t))
}

func (t ContractHash) String() string {
	return Felt(t).String()
}

func (t *ContractHash) UnmarshalJSON(data []byte) error {
	f := Felt{}
	err := f.UnmarshalJSON(data)
	if err != nil {
		return err
	}
	*t = ContractHash(f)
	return nil
}
