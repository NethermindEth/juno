package common

import "sync"

// IKey - the key of the dictionary
type IKey interface{}

// IValue - the value of the dictionary
type IValue interface{}

// Dictionary - the dictionary object with key of type IKey & vlaue of type IValue
type Dictionary struct {
	items map[IKey]IValue
	lock  sync.RWMutex
}

// Add adds a new item to the dictionary
func (dict *Dictionary) Add(key IKey, value IValue) {
	dict.lock.Lock()
	defer dict.lock.Unlock()
	if dict.items == nil {
		dict.items = make(map[IKey]IValue)
	}
	dict.items[key] = value
}

// Remove removes a value from the dictionary, given its key
func (dict *Dictionary) Remove(key IKey) bool {
	dict.lock.Lock()
	defer dict.lock.Unlock()
	_, ok := dict.items[key]
	if ok {
		delete(dict.items, key)
	}
	return ok
}

// Exist returns true if the key exists in the dictionary
func (dict *Dictionary) Exist(key IKey) bool {
	dict.lock.RLock()
	defer dict.lock.RUnlock()
	_, ok := dict.items[key]
	return ok
}

// Get returns the value associated with the key
func (dict *Dictionary) Get(key IKey) IValue {
	dict.lock.RLock()
	defer dict.lock.RUnlock()
	return dict.items[key]
}

// Clear removes all the items from the dictionary
func (dict *Dictionary) Clear() {
	dict.lock.Lock()
	defer dict.lock.Unlock()
	dict.items = make(map[IKey]IValue)
}

// Size returns the amount of elements in the dictionary
func (dict *Dictionary) Size() int {
	dict.lock.RLock()
	defer dict.lock.RUnlock()
	return len(dict.items)
}

// GetKeys returns a slice of all the keys present
func (dict *Dictionary) GetKeys() []IKey {
	dict.lock.RLock()
	defer dict.lock.RUnlock()
	var keys []IKey
	for i := range dict.items {
		keys = append(keys, i)
	}
	return keys
}

// GetValues returns a slice of all the values present
func (dict *Dictionary) GetValues() []IValue {
	dict.lock.RLock()
	defer dict.lock.RUnlock()
	var values []IValue
	for i := range dict.items {
		values = append(values, dict.items[i])
	}
	return values
}
