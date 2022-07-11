package starknet

import "sync"

// interface{} - the value of the dictionary
//type interface{} interface {
//	Marshal() ([]byte, error)
//	UnMarshal([]byte) (interface{}, error)
//}

// dictionary - the dictionary object with key of type string & value of type interface{}
type dictionary struct {
	m  map[interface{}]interface{}
	mu sync.RWMutex
}

func NewDictionary() *dictionary {
	return &dictionary{
		m: make(map[interface{}]interface{}),
	}
}

// Add adds a new item to the dictionary
func (dict *dictionary) Add(key interface{}, value interface{}) {
	dict.mu.Lock()
	dict.m[key] = value
	dict.mu.Unlock()
}

// Remove removes a value from the dictionary, given its key
func (dict *dictionary) Remove(key interface{}) {
	dict.mu.Lock()
	delete(dict.m, key)
	dict.mu.Unlock()
}

// Exist returns true if the key exists in the dictionary
func (dict *dictionary) Exist(key interface{}) bool {
	dict.mu.RLock()
	_, ok := dict.m[key]
	dict.mu.RUnlock()
	return ok
}

// Get returns the value associated with the key
func (dict *dictionary) Get(key interface{}) (interface{}, bool) {
	dict.mu.RLock()
	value, ok := dict.m[key]
	dict.mu.RUnlock()
	return value, ok
}
