package felt

type Address Felt

func (a *Address) Bytes() [32]byte {
	return (*Felt)(a).Bytes()
}

func (a *Address) String() string {
	return (*Felt)(a).String()
}

func (a *Address) UnmarshalJSON(data []byte) error {
	return (*Felt)(a).UnmarshalJSON(data)
}

func (a *Address) MarshalJSON() ([]byte, error) {
	return (*Felt)(a).MarshalJSON()
}
