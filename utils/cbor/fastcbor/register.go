package fastcbor

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
)

// RegisterFelt declares t, an array of four uint64 or a slice of them, as written as a felt or as
// an array of felts. A type that is not declared is read through its own decoder. Call it before
// the first read.
func RegisterFelt(t reflect.Type) error {
	elem := t
	if t.Kind() == reflect.Slice {
		elem = t.Elem()
	}
	if elem.Kind() != reflect.Array || elem.Len() != cursor.Limbs ||
		elem.Elem().Kind() != reflect.Uint64 {
		return fmt.Errorf("fastcbor: %v is not an array of four uint64 or a slice of them", t)
	}
	feltsMu.Lock()
	defer feltsMu.Unlock()
	feltEncoded[t] = true
	return nil
}

var feltsMu sync.RWMutex

// declaredFelt reports whether t is a type that writes itself as a felt or as an array of felts.
func declaredFelt(t reflect.Type) bool {
	feltsMu.RLock()
	defer feltsMu.RUnlock()
	return feltEncoded[t]
}

// RegisterFeltMap tells the package to read a map[K]*V of felts with a reader written for it.
func RegisterFeltMap[K, V ~limbs]() {
	registerSpecial(reflect.TypeFor[map[K]*V](), asFieldReader(readFeltMap[K, V]))
}

// RegisterFeltStorageMap tells the package to read a map[K]map[K]*V of felts with a reader written
// for it.
func RegisterFeltStorageMap[K, V ~limbs]() {
	registerSpecial(reflect.TypeFor[map[K]map[K]*V](), asFieldReader(readStorageDiffs[K, V]))
}

// RegisterFeltValueMap tells the package to read a map[K]V of felts with a reader written for it.
func RegisterFeltValueMap[K, V ~limbs]() {
	registerSpecial(reflect.TypeFor[map[K]V](), asFieldReader(readFeltValueMap[K, V]))
}

var specialMu sync.RWMutex

func registerSpecial(t reflect.Type, read fieldReader) {
	specialMu.Lock()
	defer specialMu.Unlock()
	special[t] = read
}

func specialReader(t reflect.Type) (fieldReader, bool) {
	specialMu.RLock()
	defer specialMu.RUnlock()
	read, ok := special[t]
	return read, ok
}
