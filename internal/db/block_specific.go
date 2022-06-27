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
	database Database
}

// NewBlockSpecificDatabase creates a new instance of BlockSpecificDatabase
func NewBlockSpecificDatabase(database Database) *BlockSpecificDatabase {
	return &BlockSpecificDatabase{database: database}
}

// Get sets to `value` (must be a pointer), the value associated with the given key, and block number.
// If the database does not have a value in the requested block, it sets to `value` the value in the closest
// block less than the blockNumber. Returns true if the value is found and false if not.
func (db *BlockSpecificDatabase) Get(key []byte, blockNumber uint64) ([]byte, error) {
	data := db.get(key)
	if data == nil {
		return nil, nil
	}
	// Decode the data as a sortedList
	var list sortedList
	if err := json.Unmarshal(data, &list); err != nil {
		// notest
		return nil, err
	}
	// Get the best fit
	bestFit, ok := list.Search(blockNumber)
	if !ok {
		return nil, nil
	}
	// Build the compound key
	keyRaw := newCompoundedKey(key, bestFit)
	// Get the element
	data = db.get(keyRaw)
	if data == nil {
		// notest
		return nil, fmt.Errorf("unexpected not found for key %s at block %d", string(keyRaw), blockNumber)
	}
	return data, nil
}

// Put stores the given value at the tuple (key,blockNumber).
func (db *BlockSpecificDatabase) Put(key []byte, blockNumber uint64, value []byte) error {
	rawList := db.get(key)
	var list sortedList
	if rawList != nil {
		if err := json.Unmarshal(rawList, &list); err != nil {
			// notest
			return err
		}
	}
	list.Add(blockNumber)
	newRawList, err := json.Marshal(&list)
	if err != nil {
		// notest
		return err
	}
	err = db.database.Put(key, newRawList)
	if err != nil {
		return err
	}

	rawKey := newCompoundedKey(key, blockNumber)
	err = db.database.Put(rawKey, value)
	if err != nil {
		return err
	}
	return nil
}

func (db *BlockSpecificDatabase) Close() {
	db.database.Close()
}

func (db *BlockSpecificDatabase) get(key []byte) []byte {
	data, err := db.database.Get(key)
	if err != nil {
		// notest
		if IsNotFound(err) {
			return nil
		}
		panicWithError(err)
	}
	return data
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
