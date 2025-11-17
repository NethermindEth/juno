package felt

type Hash Felt

func (h *Hash) Bytes() [32]byte {
	return (*Felt)(h).Bytes()
}

func (h *Hash) String() string {
	return (*Felt)(h).String()
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	return (*Felt)(h).UnmarshalJSON(data)
}

func (h *Hash) MarshalJSON() ([]byte, error) {
	return (*Felt)(h).MarshalJSON()
}

func (h *Hash) Marshal() []byte {
	return (*Felt)(h).Marshal()
}

func (h *Hash) Unmarshal(e []byte) {
	(*Felt)(h).Unmarshal(e)
}

type ClassHash Hash

func (h *ClassHash) String() string {
	return (*Hash)(h).String()
}

func (h *ClassHash) UnmarshalJSON(data []byte) error {
	return (*Hash)(h).UnmarshalJSON(data)
}

func (h *ClassHash) MarshalJSON() ([]byte, error) {
	return (*Hash)(h).MarshalJSON()
}

func (h *ClassHash) Marshal() []byte {
	return (*Hash)(h).Marshal()
}

func (h *ClassHash) Unmarshal(e []byte) {
	(*Hash)(h).Unmarshal(e)
}

type SierraClassHash ClassHash

func (h *SierraClassHash) String() string {
	return (*ClassHash)(h).String()
}

func (h *SierraClassHash) UnmarshalJSON(data []byte) error {
	return (*ClassHash)(h).UnmarshalJSON(data)
}

func (h *SierraClassHash) MarshalJSON() ([]byte, error) {
	return (*ClassHash)(h).MarshalJSON()
}

func (h *SierraClassHash) Marshal() []byte {
	return (*ClassHash)(h).Marshal()
}

func (h *SierraClassHash) Unmarshal(e []byte) {
	(*ClassHash)(h).Unmarshal(e)
}

type CasmClassHash ClassHash

func (h *CasmClassHash) String() string {
	return (*ClassHash)(h).String()
}

func (h *CasmClassHash) UnmarshalJSON(data []byte) error {
	return (*ClassHash)(h).UnmarshalJSON(data)
}

func (h *CasmClassHash) MarshalJSON() ([]byte, error) {
	return (*ClassHash)(h).MarshalJSON()
}

func (h *CasmClassHash) Marshal() []byte {
	return (*ClassHash)(h).Marshal()
}

func (h *CasmClassHash) Unmarshal(e []byte) {
	(*ClassHash)(h).Unmarshal(e)
}

type TransactionHash Hash

func (h *TransactionHash) String() string {
	return (*Hash)(h).String()
}

func (h *TransactionHash) UnmarshalJSON(data []byte) error {
	return (*Hash)(h).UnmarshalJSON(data)
}

func (h *TransactionHash) MarshalJSON() ([]byte, error) {
	return (*Hash)(h).MarshalJSON()
}
