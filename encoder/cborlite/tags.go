package cborlite

import (
	"reflect"
	"sync"
)

var tagTypes sync.Map // uint64 -> reflect.Type

// RegisterTag records which concrete type a CBOR tag stands for.
// The numbers come from encoder.RegisterType.
func RegisterTag(tag uint64, concrete reflect.Type) {
	tagTypes.Store(tag, concrete)
}
