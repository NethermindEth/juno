package types

import (
	"github.com/NethermindEth/juno/pkg/db"
)

// IKey - the key of the dictionary
type IKey interface {
	Marshal() ([]byte, error)
}

// IValue - the value of the dictionary
type IValue interface {
	Marshal() ([]byte, error)
	UnMarshal([]byte) (IValue, error)
}

// Dictionary - the dictionary object with key of type IKey & value of type IValue
type Dictionary struct {
	database db.Databaser
	prefix   []byte
}

func NewDictionary(database db.Databaser, prefix string) *Dictionary {
	return &Dictionary{
		database: database,
		prefix:   []byte(prefix),
	}
}

// Add adds a new item to the dictionary
func (dict *Dictionary) Add(key string, value IValue) {
	v, err := value.Marshal()
	if err != nil {
		// notest
		return
	}
	err = dict.database.Put(append(dict.prefix, []byte(key)...), v)
	if err != nil {
		// notest
		return
	}
}

// Remove removes a value from the dictionary, given its key
func (dict *Dictionary) Remove(key string) bool {
	err := dict.database.Delete(append(dict.prefix, []byte(key)...))
	return err == nil
}

// Exist returns true if the key exists in the dictionary
func (dict *Dictionary) Exist(key string) bool {
	has, err := dict.database.Has(append(dict.prefix, []byte(key)...))
	if err != nil {
		// notest
		return false
	}
	return has
}

// Get returns the value associated with the key
func (dict *Dictionary) Get(key string, value IValue) (IValue, error) {
	val, err := dict.database.Get(append(dict.prefix, []byte(key)...))
	if err != nil {
		return value, err
	}
	marshal, err := value.UnMarshal(val)
	if err != nil {
		return nil, err
	}
	return marshal, nil
}
