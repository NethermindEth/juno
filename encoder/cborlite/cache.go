package cborlite

import (
	"reflect"
	"sync"
)

const maxNestedPlans = 32

type reader func(target reflect.Value, data []byte) (consumed int, err error)

// builtReader is what a build produced, including an error.
type builtReader struct {
	reader
	err error
}

// cacheKey identifies a reader by type and strictness.
type cacheKey struct {
	valueType reflect.Type
	strict    bool
}

var (
	// Finished readers
	readers sync.Map // cacheKey -> builtReader

	// buildMutex serialises building.
	buildMutex sync.Mutex
	buildDepth int

	// Finished plans.
	plans = map[cacheKey]*plan{}
)

// cachedReader builds a reader and cache it so it only builds once.
func cachedReader(valueType reflect.Type, strict bool) (reader, error) {
	key := cacheKey{valueType: valueType, strict: strict}

	cached, ok := readers.Load(key)
	if ok {
		built := cached.(builtReader)
		return built.reader, built.err
	}

	buildMutex.Lock()
	defer buildMutex.Unlock()

	// Another goroutine may have finished while this one waited.
	cached, ok = readers.Load(key)
	if ok {
		built := cached.(builtReader)
		return built.reader, built.err
	}

	built, err := buildReader(valueType, strict)
	readers.Store(key, builtReader{reader: built, err: err})
	return built, err
}

func buildReader(valueType reflect.Type, strict bool) (reader, error) {
	// If the type reads itself (e.g. implements UnmarshalCBORPrefix), it takes priority
	if read, ok := specialTypeReader(valueType); ok {
		return read, nil
	}
	return kindReader(valueType, strict)
}
