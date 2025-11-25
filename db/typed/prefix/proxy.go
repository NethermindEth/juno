package prefix

import "github.com/NethermindEth/juno/db/typed/key"

type proxy[B any, T tag] struct{}

func Prefix[B any, H any, HS key.Serializer[H], T tag](
	head HS,
	tail proxy[B, T],
) proxy[B, hasPrefix[B, H, HS, T]] {
	return proxy[B, hasPrefix[B, H, HS, T]]{}
}

func End[B any]() proxy[B, end[B]] {
	return proxy[B, end[B]]{}
}
