package main

import (
	"log"

	"github.com/NethermindEth/juno/feedergatewaysync"
)

func main() {
	// Define feeder-gateway URL for Sepolia
	feederURL := "https://alpha-sepolia.starknet.io"

	// Initialize PebbleDB storage
	dbPath := "pebble_data"
	storage, err := feedergatewaysync.NewStorage(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer storage.Close()

	// Create sync manager
	syncManager := feedergatewaysync.NewSyncManager(feederURL, storage)

	// Define block range to sync
	startBlock := uint64(1)
	endBlock := uint64(500)

	// Start syncing blocks
	log.Println("Starting block sync...")
	syncManager.SyncBlocks(startBlock, endBlock)
	log.Printf("%d nodes synced successfully", endBlock-startBlock+1)
}
