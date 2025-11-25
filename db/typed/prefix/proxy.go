package prefix

import "github.com/NethermindEth/juno/db/typed/key"

type proxy[V any, T tag[V]] struct{}

func Prefix[V any, H any, HS key.Serializer[H], T tag[V]](
	head HS,
	tail proxy[V, T],
) proxy[V, hasPrefix[V, H, HS, T]] {
	return proxy[V, hasPrefix[V, H, HS, T]]{}
}

func End[V any]() proxy[V, end[V]] {
	return proxy[V, end[V]]{}
}
