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

func (a *Address) Marshal() []byte {
	return (*Felt)(a).Marshal()
}

func (a *Address) Unmarshal(e []byte) {
	(*Felt)(a).Unmarshal(e)
}

func (a *Address) SetBytesCanonical(data []byte) error {
	return (*Felt)(a).SetBytesCanonical(data)
}

func (a *Address) IsZero() bool {
	return (*Felt)(a).IsZero()
}

func (a *Address) Equal(b *Address) bool {
	return (*Felt)(a).Equal((*Felt)(b))
}
