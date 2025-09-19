package felt

type Address Felt

func (a *Address) String() string {
	return (*Felt)(a).String()
}
