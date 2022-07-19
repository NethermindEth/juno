package services

import (
	"context"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db/sync"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
)

var APICollector *apiCollector

type apiCollector struct {
	service
	// manager is the Sync manager
	manager *sync.Manager
	// feeder is the client that provides the Connection to the feeder gateway.
	client *feeder.Client
	// close is the channel that will be used to close the collector.
	chainID int
	// buffer represent the channel of StateDiff collected
	buffer chan *types.StateDiff
	// latestBlockOnChain is the last block on chain that need to be collected
	latestBlockOnChain int64
	// Synced is the flag that indicate if the collector is synced
	synced bool
}

func NewApiCollector(manager *sync.Manager, feeder *feeder.Client, chainID int) {
	APICollector = &apiCollector{
		client:  feeder,
		manager: manager,
		chainID: chainID,
	}
	APICollector.logger = Logger.Named("apiCollector")
	APICollector.buffer = make(chan *types.StateDiff, 10)
	APICollector.synced = false
	go APICollector.updateLatestBlockOnChain()
}

// Run start to store StateDiff locally
func (a *apiCollector) Run() error {
	a.logger.Info("Service Started")
	// start the buffer updater
	latestStateDiffSynced := a.manager.GetLatestBlockSync()
	for {
		if latestStateDiffSynced >= a.latestBlockOnChain {
			a.synced = true
			time.Sleep(time.Second * 3)
		}
		var update *feeder.StateUpdateResponse
		var err error
		if a.chainID == 1 { // mainnet
			update, err = a.client.GetStateUpdate("", strconv.FormatInt(latestStateDiffSynced, 10))
			if err != nil {
				a.logger.With("Error", err, "Block Number", latestStateDiffSynced).Info("Couldn't get state update")
				continue
			}
		} else { // goerli
			update, err = a.client.GetStateUpdateGoerli("", strconv.FormatInt(latestStateDiffSynced, 10))
			if err != nil {
				a.logger.With("Error", err).Info("Couldn't get state update")
				continue
			}
		}
		a.buffer <- stateUpdateResponseToStateDiff(*update, latestStateDiffSynced)
		a.logger.With("BlockNumber", latestStateDiffSynced).Info("StateDiff collected")
		latestStateDiffSynced += 1
	}
}

func (a *apiCollector) Close(context.Context) {
	close(a.buffer)
}

// GetChannel returns the channel of StateDiffs
func (a *apiCollector) GetChannel() chan *types.StateDiff {
	return a.buffer
}

// updateLatestBlockOnChain update the latest block on chain
func (a *apiCollector) updateLatestBlockOnChain() {
	a.latestBlockOnChain = a.fetchLatestBlockOnChain()
	for {
		time.Sleep(time.Minute)
		a.latestBlockOnChain = a.fetchLatestBlockOnChain()
	}
}

// fetchLatestBlockOnChain fetch the latest block on chain
func (a *apiCollector) fetchLatestBlockOnChain() int64 {
	pendingBlock, err := a.client.GetBlock("", "pending")
	if err != nil {
		return 0
	}
	parentBlock, err := a.client.GetBlock(pendingBlock.ParentBlockHash, "")
	if err != nil {
		return 0
	}
	return int64(parentBlock.BlockNumber)
}

func (a *apiCollector) GetLatestBlockOnChain() int64 {
	return a.latestBlockOnChain
}

func (a *apiCollector) IsSynced() bool {
	return a.synced
}

// stateUpdateResponseToStateDiff convert the input feeder.StateUpdateResponse to StateDiff
func stateUpdateResponseToStateDiff(update feeder.StateUpdateResponse, blockNumber int64) *types.StateDiff {
	var stateDiff types.StateDiff
	stateDiff.NewRoot = new(felt.Felt).SetHex(update.NewRoot)
	stateDiff.BlockNumber = blockNumber
	stateDiff.OldRoot = new(felt.Felt).SetHex(update.OldRoot)
	stateDiff.DeployedContracts = make([]types.DeployedContract, len(update.StateDiff.DeployedContracts))
	for i, v := range update.StateDiff.DeployedContracts {
		stateDiff.DeployedContracts[i] = types.DeployedContract{
			Address: new(felt.Felt).SetHex(v.Address),
			Hash:    new(felt.Felt).SetHex(v.ContractHash),
		}
	}
	stateDiff.StorageDiff = make(types.StorageDiff)
	for contractAddress, memoryCells := range update.StateDiff.StorageDiffs {
		kvs := make([]types.MemoryCell, 0)
		for _, cell := range memoryCells {
			kvs = append(kvs, types.MemoryCell{
				Address: new(felt.Felt).SetHex(cell.Key),
				Value:   new(felt.Felt).SetHex(cell.Value),
			})
		}
		// Create felt and convert to string for consistency
		stateDiff.StorageDiff[new(felt.Felt).SetHex(contractAddress).String()] = kvs
	}

	return &stateDiff
}
