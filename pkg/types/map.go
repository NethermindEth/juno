package types

import "sync"

// IValue - the value of the dictionary
type IValue interface {
	Marshal() ([]byte, error)
	UnMarshal([]byte) (IValue, error)
}

// Dictionary - the dictionary object with key of type interface{} & value of type IValue
type Dictionary struct {
	database map[interface{}]interface{}
	mutex    sync.Mutex
}

func NewDictionary() *Dictionary {
	return &Dictionary{
		database: make(map[interface{}]interface{}),
	}
}

// Add adds a new item to the dictionary
func (dict *Dictionary) Add(key interface{}, value IValue) {
	dict.mutex.Lock()
	dict.database[key] = value
	dict.mutex.Unlock()
}

// Remove removes a value from the dictionary, given its key
func (dict *Dictionary) Remove(key interface{}) {
	dict.mutex.Lock()
	delete(dict.database, key)
	dict.mutex.Unlock()
}

// Exist returns true if the key exists in the dictionary
func (dict *Dictionary) Exist(key interface{}) bool {
	dict.mutex.Lock()
	_, ok := dict.database[key]
	dict.mutex.Unlock()
	return ok
}

// Get returns the value associated with the key
func (dict *Dictionary) Get(key interface{}) (interface{}, bool) {
	dict.mutex.Lock()
	value, ok := dict.database[key]
	dict.mutex.Unlock()
	return value, ok
}
