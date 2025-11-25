package prefix

import (
	"github.com/NethermindEth/juno/db/typed/key"
)

type tag interface {
	~[]byte
}

type hasPrefix[B any, H any, HS key.Serializer[H], T tag] []byte

func (h hasPrefix[B, H, HS, T]) Add(head H) T {
	var hs HS
	return T(append(h, hs.Marshal(head)...))
}

func (h hasPrefix[B, H, HS, T]) Finish() end[B] {
	return end[B](h)
}

type end[B any] []byte
