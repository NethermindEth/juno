package feedergatewaysync

import (
	"log"
)

// SyncManager handles block fetching and storage
type SyncManager struct {
	feederURL string
	storage   *Storage
}

// NewSyncManager initializes the sync manager
func NewSyncManager(feederURL string, storage *Storage) *SyncManager {
	return &SyncManager{
		feederURL: feederURL,
		storage:   storage,
	}
}

// SyncBlocks fetches blocks from the feeder and stores them
func (s *SyncManager) SyncBlocks(startBlock, endBlock uint64) {
	for i := startBlock; i <= endBlock; i++ {
		block, err := s.FetchBlock(i)
		if err != nil {
			log.Printf("Error fetching block %d: %v", i, err)
			continue
		}

		// Save block in PebbleDB
		err = s.storage.SaveBlock(block)
		if err != nil {
			log.Printf("Error saving block %d: %v", i, err)
		}
	}
	log.Println("Syncing completed!")
}
