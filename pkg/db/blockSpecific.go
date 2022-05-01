package db

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/internal/log"
)

// BlockSpecificDatabase is a database to store values that must have a history of versions on the blockchain.
// To get and put objects in the database is needed the key and a block number.
type BlockSpecificDatabase struct {
	database Databaser
}

// Key is an interface to represent a key in the database.
// A database key must be able to marshal itself to an array of bytes.
type Key interface {
	Marshal() ([]byte, error)
}

// Value is an interface to represent a value in the database.
// A database value must be able to marshal and unmarshal itself from an array of bytes.
type Value interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

// NewBlockSpecificDatabase creates a new instance of BlockSpecificDatabase
func NewBlockSpecificDatabase(database Databaser) *BlockSpecificDatabase {
	return &BlockSpecificDatabase{database: database}
}

// Get sets to `value` (must be a pointer), the value associated with the given key, and block number.
// If the database does not have a value in the requested block, it sets to `value` the value in the closest
// block less than the blockNumber. Returns true if the value is found and false if not.
func (db *BlockSpecificDatabase) Get(key Key, blockNumber uint64, value Value) bool {
	// Get the sortedList of block versions
	keyRaw, err := key.Marshal()
	if err != nil {
		// notest
		panicWithError(err)
	}
	data := db.get(keyRaw)
	if data == nil {
		return false
	}
	// Decode the data as a sortedList
	var list sortedList
	if err := json.Unmarshal(data, &list); err != nil {
		// notest
		panicWithError(err)
	}
	// Get the best fit
	bestFit, ok := list.Search(blockNumber)
	if !ok {
		return false
	}
	// Build the compound key
	keyRaw = newCompoundedKey(keyRaw, bestFit)
	// Get the element
	data = db.get(keyRaw)
	if data == nil {
		// notest
		panicWithError(fmt.Errorf("unexpected not found for key %s at block %d", string(keyRaw), blockNumber))
	}
	if err := value.Unmarshal(data); err != nil {
		// notest
		panicWithError(err)
	}
	return true
}

// Put stores the given value at the tuple (key,blockNumber).
func (db *BlockSpecificDatabase) Put(key Key, blockNumber uint64, value Value) {
	// Create/Update the sortedList
	rawKey, err := key.Marshal()
	if err != nil {
		// notest
		panicWithError(err)
	}
	rawList := db.get(rawKey)
	var list sortedList
	if rawList != nil {
		if err := json.Unmarshal(rawList, &list); err != nil {
			// notest
			panicWithError(err)
		}
	}
	list.Add(blockNumber)
	newRawList, err := json.Marshal(&list)
	if err != nil {
		// notest
		panicWithError(err)
	}
	db.put(rawKey, newRawList)

	rawKey = newCompoundedKey(rawKey, blockNumber)
	rawValue, err := value.Marshal()
	if err != nil {
		// notest
		panicWithError(err)
	}
	db.put(rawKey, rawValue)
}

func (db *BlockSpecificDatabase) get(key []byte) []byte {
	data, err := db.database.Get(key)
	if err != nil {
		// notest
		panicWithError(err)
	}
	return data
}

func (db *BlockSpecificDatabase) put(key []byte, value []byte) {
	err := db.database.Put(key, value)
	if err != nil {
		// notest
		panicWithError(err)
	}
}

func newCompoundedKey(key []byte, blockNumber uint64) []byte {
	key = append(key, '.')
	bn := make([]byte, 8)
	binary.LittleEndian.PutUint64(bn, blockNumber)
	return append(key, bn...)
}

func panicWithError(err error) {
	log.Default.With("error", err).Error("unexpected error")
	panic(any(err))
}
