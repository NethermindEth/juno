package feedergatewaysync

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
)

// Storage handles reading/writing data to PebbleDB
type Storage struct {
	db *pebble.DB
}

// NewStorage initializes a new PebbleDB instance
func NewStorage(dbPath string) (*Storage, error) {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to open PebbleDB: %w", err)
	}
	return &Storage{db: db}, nil
}

// SaveBlock stores a CustomBlock in PebbleDB
func (s *Storage) SaveBlock(block *CustomBlock) error {
	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to serialize block: %w", err)
	}
	blockKey := fmt.Sprintf("block-%d", block.Number)
	err = s.db.Set([]byte(blockKey), data, pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to store block in PebbleDB: %w", err)
	}
	log.Printf("Block %d stored successfully", block.Number)
	return nil
}

// GetBlock retrieves a CustomBlock from PebbleDB
func (s *Storage) GetBlock(blockNumber uint64) (*CustomBlock, error) {
	blockKey := fmt.Sprintf("block-%d", blockNumber)
	data, closer, err := s.db.Get([]byte(blockKey))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("block %d not found", blockNumber)
	} else if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	defer closer.Close()

	var block CustomBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("failed to deserialize block: %w", err)
	}
	return &block, nil
}

// Close closes the PebbleDB instance
func (s *Storage) Close() error {
	return s.db.Close()
}
