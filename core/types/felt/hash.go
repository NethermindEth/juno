package felt

type Hash Felt

func (h *Hash) String() string {
	return (*Felt)(h).String()
}

type ClassHash Hash

func (h *ClassHash) String() string {
	return (*Hash)(h).String()
}

type TransactionHash Hash

func (h *TransactionHash) String() string {
	return (*Hash)(h).String()
}
