package encoder

// SelfEncoder collects the encode hook of every engine.
// A type that wants to be hand-encoded has to satisfy all of them.
type SelfEncoder interface {
	MarshalCBOR() ([]byte, error) // fxamacker
}

// SelfDecoder collects the decode hook of every engine.
// A type that wants to be hand-decoded has to satisfy all of them.
type SelfDecoder interface {
	UnmarshalCBOR([]byte) error // fxamacker
}
