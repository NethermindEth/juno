package felt

type Hash Felt

func (h *Hash) Bytes() [32]byte {
	return (*Felt)(h).Bytes()
}

func (h *Hash) String() string {
	return (*Felt)(h).String()
}

type ClassHash Hash

func (h *ClassHash) String() string {
	return (*Hash)(h).String()
}

type SierraClassHash ClassHash

func (h *SierraClassHash) String() string {
	return (*ClassHash)(h).String()
}

func (h *SierraClassHash) UnmarshalJSON(data []byte) error {
	return (*Felt)(h).UnmarshalJSON(data)
}

func (h *SierraClassHash) MarshalJSON() ([]byte, error) {
	return (*Felt)(h).MarshalJSON()
}

type CasmClassHash ClassHash

func (h *CasmClassHash) String() string {
	return (*ClassHash)(h).String()
}

func (h *CasmClassHash) UnmarshalJSON(data []byte) error {
	return (*Felt)(h).UnmarshalJSON(data)
}

func (h *CasmClassHash) MarshalJSON() ([]byte, error) {
	return (*Felt)(h).MarshalJSON()
}

type TransactionHash Hash

func (h *TransactionHash) String() string {
	return (*Hash)(h).String()
}
