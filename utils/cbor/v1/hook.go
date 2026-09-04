package cbor

// SelfEncoder is the hook a type implements to write its own encoding.
type SelfEncoder interface {
	MarshalCBOR() ([]byte, error) // fxamacker
}

// SelfDecoder is the hook a type implements to read its own encoding.
type SelfDecoder interface {
	UnmarshalCBOR([]byte) error // fxamacker
}
