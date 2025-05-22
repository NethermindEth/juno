package types

type Timeout struct {
	Step   Step
	Height Height
	Round  Round
	_      struct{} `cbor:",toarray"`
}

func (t Timeout) MsgType() MessageType {
	return MessageTypeTimeout
}

func (t Timeout) GetHeight() Height {
	return t.Height
}
