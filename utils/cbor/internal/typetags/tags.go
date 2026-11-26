// Package typetags exposes the wire tags assigned at startup to CBOR decoder planners.
package typetags

import (
	"iter"
	"maps"
	"reflect"
)

// Registration finishes during package initialization, before any decoder plans are built.
var registered = make(map[reflect.Type]uint64)

// Record saves the tag successfully assigned to t by the encoding adapter.
func Record(t reflect.Type, tag uint64) { registered[t] = tag }

// Registered iterates over concrete types and their wire tags after initialization.
func Registered() iter.Seq2[reflect.Type, uint64] { return maps.All(registered) }
