// Package starknet contains all the functions related to Ethereum State and Synchronization
// with Layer 1
package starknet

import (
	"context"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/feeder_gateway"
	"strconv"
	"time"
)

const (
	LatestBlockSync = "starknet_latest_block_sync"
)

// Synchronizer represents the base struct for StarkNet Synchronization
type Synchronizer struct {
	client *feeder_gateway.Client
	db     *db.Databaser
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(feederGateway string, db *db.Databaser) *Synchronizer {
	client := feeder_gateway.NewClient(feederGateway, "/feeder_gateway", nil)
	return &Synchronizer{
		client: client,
		db:     db,
	}
}

// UpdateStateRoot keeps updating the Starknet State Root as a process
func (s Synchronizer) UpdateStateRoot() error {
	// Check since which block we should start to update the StarkNet State
	value, err := (*s.db).Get([]byte(LatestBlockSync))
	if err != nil {
		return err
	}
	latestBlockSynced := 0
	if value != nil {
		latestBlockSynced, err = strconv.Atoi(string(value))
	}
	defer func() {
		err := (*s.db).Put([]byte(LatestBlockSync), []byte(strconv.Itoa(latestBlockSynced)))
		if err != nil {
			log.Default.With("Error", err).Info("Error saving the latest block sync from Starknet Gateway")
		}
	}()
	log.Default.With("Latest Block ", latestBlockSynced).Info("Got the latest block synced")

	starknetBlock, err := s.client.GetBlock("", "")
	if err != nil {
		return err
	}
	var latestBlockHash string
	for i := latestBlockSynced; i < starknetBlock.BlockNumber; i++ {
		update, err := s.client.GetStateUpdate("", strconv.Itoa(i))
		if err != nil {
			return err
		}
		// Update state
		s.updateState(update)
	}
	for _ = range time.Tick(time.Second * 1) {
		newStateUpdate, err := s.client.GetStateUpdate("", "")
		if err != nil {
			log.Default.With("Error", err).Info("")
		}
		if newStateUpdate.BlockHash != latestBlockHash {
			s.updateState(newStateUpdate)
		}
	}
	return nil
}

// Close closes the client for the Layer 1 Ethereum node
func (s Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	select {
	case <-ctx.Done():
	default:
	}
}

func (s Synchronizer) updateState(stateUpdate feeder_gateway.StateUpdateResponse) {
	log.Default.With("Block Hash", stateUpdate.BlockHash, "New Root", stateUpdate.NewRoot, "Old Root",
		stateUpdate.OldRoot).Info("Updating Starknet State")
}
