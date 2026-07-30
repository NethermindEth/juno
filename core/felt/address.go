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

func (a Address) MarshalText() ([]byte, error) {
	return Felt(a).MarshalText()
}

func (a Address) AppendText(data []byte) ([]byte, error) {
	return Felt(a).AppendText(data)
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
