package prefix

import (
	"slices"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
)

type tag[V any] interface {
	~struct {
		scanState[V]
	}
}

type hasPrefix[V any, H any, HS key.Serializer[H], T tag[V]] struct {
	scanState[V]
}

func (h hasPrefix[V, H, HS, T]) Add(head H) T {
	return T{
		scanState: scanState[V]{
			prefix:  append(h.prefix, HS{}.Marshal(head)...),
			scanner: h.scanner,
		},
	}
}

func (h hasPrefix[V, H, HS, T]) DeleteRange(
	database db.KeyValueRangeDeleter,
	startKey,
	endKey H,
) error {
	return database.DeleteRange(
		append(h.prefix, HS{}.Marshal(startKey)...),
		append(slices.Clone(h.prefix), HS{}.Marshal(endKey)...),
	)
}

type end[V any] struct {
	scanState[V]
}
