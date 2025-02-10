package main

import (
	"log"

	"github.com/NethermindEth/juno/feedergatewaysync"
)

const (
	feederURL  = "https://alpha-sepolia.starknet.io"
	dbPath     = "pebble_data"
	startBlock = 1
	endBlock   = 500
)

func main() {
	// Initialise PebbleDB storage
	storage, err := feedergatewaysync.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialise storage: %v", err)
	}
	defer storage.Close()

	// Create sync manager
	syncManager := feedergatewaysync.NewSyncManager(feederURL, storage)

	// Start syncing blocks
	log.Println("Starting block sync...")
	syncManager.SyncBlocks(startBlock, endBlock)
	log.Printf("%d nodes synced successfully", endBlock-startBlock+1)
}
