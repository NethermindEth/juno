package felt

import "encoding/json/jsontext"

type Hash Felt

func (h *Hash) Bytes() [32]byte {
	return (*Felt)(h).Bytes()
}

func (h *Hash) String() string {
	return (*Felt)(h).String()
}

func (h *Hash) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return (*Felt)(h).UnmarshalJSONFrom(dec)
}

func (h *Hash) UnmarshalText(text []byte) error {
	return (*Felt)(h).UnmarshalText(text)
}

func (h Hash) MarshalText() ([]byte, error) {
	return Felt(h).MarshalText()
}

func (h Hash) AppendText(data []byte) ([]byte, error) {
	return Felt(h).AppendText(data)
}

func (h *Hash) Marshal() []byte {
	return (*Felt)(h).Marshal()
}

func (h *Hash) Unmarshal(e []byte) {
	(*Felt)(h).Unmarshal(e)
}

func (h *Hash) SetBytesCanonical(data []byte) error {
	return (*Felt)(h).SetBytesCanonical(data)
}

type ClassHash Hash

func (h *ClassHash) String() string {
	return (*Hash)(h).String()
}

func (h *ClassHash) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return (*Hash)(h).UnmarshalJSONFrom(dec)
}

func (h *ClassHash) UnmarshalText(text []byte) error {
	return (*Hash)(h).UnmarshalText(text)
}

func (h ClassHash) MarshalText() ([]byte, error) {
	return Hash(h).MarshalText()
}

func (h ClassHash) AppendText(data []byte) ([]byte, error) {
	return Hash(h).AppendText(data)
}

func (h *ClassHash) Marshal() []byte {
	return (*Hash)(h).Marshal()
}

func (h *ClassHash) Unmarshal(e []byte) {
	(*Hash)(h).Unmarshal(e)
}

func (h *ClassHash) SetBytesCanonical(data []byte) error {
	return (*Hash)(h).SetBytesCanonical(data)
}

type SierraClassHash ClassHash

func (h *SierraClassHash) String() string {
	return (*ClassHash)(h).String()
}

func (h *SierraClassHash) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return (*ClassHash)(h).UnmarshalJSONFrom(dec)
}

func (h *SierraClassHash) UnmarshalText(text []byte) error {
	return (*ClassHash)(h).UnmarshalText(text)
}

func (h SierraClassHash) MarshalText() ([]byte, error) {
	return ClassHash(h).MarshalText()
}

func (h SierraClassHash) AppendText(data []byte) ([]byte, error) {
	return ClassHash(h).AppendText(data)
}

func (h *SierraClassHash) Marshal() []byte {
	return (*ClassHash)(h).Marshal()
}

func (h *SierraClassHash) Unmarshal(e []byte) {
	(*ClassHash)(h).Unmarshal(e)
}

func (h *SierraClassHash) SetBytesCanonical(data []byte) error {
	return (*ClassHash)(h).SetBytesCanonical(data)
}

type CasmClassHash ClassHash

func (h *CasmClassHash) String() string {
	return (*ClassHash)(h).String()
}

func (h *CasmClassHash) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return (*ClassHash)(h).UnmarshalJSONFrom(dec)
}

func (h *CasmClassHash) UnmarshalText(text []byte) error {
	return (*ClassHash)(h).UnmarshalText(text)
}

func (h CasmClassHash) MarshalText() ([]byte, error) {
	return ClassHash(h).MarshalText()
}

func (h CasmClassHash) AppendText(data []byte) ([]byte, error) {
	return ClassHash(h).AppendText(data)
}

func (h *CasmClassHash) Marshal() []byte {
	return (*ClassHash)(h).Marshal()
}

func (h *CasmClassHash) Unmarshal(e []byte) {
	(*ClassHash)(h).Unmarshal(e)
}

func (h *CasmClassHash) SetBytesCanonical(data []byte) error {
	return (*ClassHash)(h).SetBytesCanonical(data)
}

type TransactionHash Hash

func (h *TransactionHash) String() string {
	return (*Hash)(h).String()
}

func (h *TransactionHash) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return (*Hash)(h).UnmarshalJSONFrom(dec)
}

func (h *TransactionHash) UnmarshalText(text []byte) error {
	return (*Hash)(h).UnmarshalText(text)
}

func (h TransactionHash) MarshalText() ([]byte, error) {
	return Hash(h).MarshalText()
}

func (h TransactionHash) AppendText(data []byte) ([]byte, error) {
	return Hash(h).AppendText(data)
}

func (h *TransactionHash) Marshal() []byte {
	return (*Hash)(h).Marshal()
}

func (h *TransactionHash) Unmarshal(e []byte) {
	(*Hash)(h).Unmarshal(e)
}

func (h *TransactionHash) SetBytesCanonical(data []byte) error {
	return (*Hash)(h).SetBytesCanonical(data)
}

type StateRootHash Hash

func (h *StateRootHash) String() string {
	return (*Hash)(h).String()
}

func (h *StateRootHash) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	return (*Hash)(h).UnmarshalJSONFrom(dec)
}

func (h *StateRootHash) UnmarshalText(text []byte) error {
	return (*Hash)(h).UnmarshalText(text)
}

func (h StateRootHash) MarshalText() ([]byte, error) {
	return Hash(h).MarshalText()
}

func (h StateRootHash) AppendText(data []byte) ([]byte, error) {
	return Hash(h).AppendText(data)
}

func (h *StateRootHash) Marshal() []byte {
	return (*Hash)(h).Marshal()
}

func (h *StateRootHash) Unmarshal(e []byte) {
	(*Hash)(h).Unmarshal(e)
}

func (h *StateRootHash) SetBytesCanonical(data []byte) error {
	return (*Hash)(h).SetBytesCanonical(data)
}
