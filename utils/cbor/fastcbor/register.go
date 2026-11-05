package fastcbor

import (
	"maps"
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
)

var (
	tagsMu sync.RWMutex
	// tags has the tag that comes before a value of each registered type.
	tags = map[reflect.Type]uint64{}
)

// RegisterType tells the package that a value of the struct type t is written after tag, so that it
// can read an interface that holds t. Call it before the first read.
func RegisterType(t reflect.Type, tag uint64) {
	tagsMu.Lock()
	defer tagsMu.Unlock()
	tags[t] = tag
}

// TagOf returns the tag that starts the encoded item.
func TagOf(encoded []byte) (uint64, error) {
	c := cursor.NewCursor(encoded)
	return c.Tag()
}

func registeredTypes() map[reflect.Type]uint64 {
	tagsMu.RLock()
	defer tagsMu.RUnlock()
	return maps.Clone(tags)
}
