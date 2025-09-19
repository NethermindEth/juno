package felt

type Address Felt

func (a *Address) Bytes() [32]byte {
	return (*Felt)(a).Bytes()
}

func (a *Address) String() string {
	return (*Felt)(a).String()
}
