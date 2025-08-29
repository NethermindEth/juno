package types

type Timeout struct {
	Step   Step
	Height Height
	Round  Round
	_      struct{} `cbor:",toarray"`
}
