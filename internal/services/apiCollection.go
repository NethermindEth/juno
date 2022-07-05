package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"strconv"
	"time"
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
	buffer chan *starknetTypes.StateDiff
	// latestBlockOnChain is the last block on chain that need to be collected
	latestBlockOnChain int64
}

func NewApiCollector(manager *sync.Manager, feeder *feeder.Client, chainID int) {
	APICollector = &apiCollector{
		client:  feeder,
		manager: manager,
		chainID: chainID,
	}
	APICollector.logger = log.Default.Named("apiCollector")
	APICollector.buffer = make(chan *starknetTypes.StateDiff, 10)
	go APICollector.updateLatestBlockOnChain()
}

// Run start to store StateDiff locally
func (a *apiCollector) Run() error {
	a.logger.Info("Service Started")
	// start the buffer updater
	latestStateDiffSynced := a.manager.GetLatestBlockSync()
	for {
		if latestStateDiffSynced >= a.latestBlockOnChain {

			time.Sleep(time.Second * 3)
		}
		var update *feeder.StateUpdateResponse
		var err error
		if a.chainID == 1 {
			update, err = a.client.GetStateUpdate("", strconv.FormatInt(latestStateDiffSynced, 10))
			if err != nil {
				a.logger.With("Error", err, "Block Number", latestStateDiffSynced).Info("Couldn't get state update")
				continue
			}
		} else {
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
func (a *apiCollector) GetChannel() chan *starknetTypes.StateDiff {
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

// stateUpdateResponseToStateDiff convert the input feeder.StateUpdateResponse to StateDiff
func stateUpdateResponseToStateDiff(update feeder.StateUpdateResponse, blockNumber int64) *starknetTypes.StateDiff {
	var stateDiff starknetTypes.StateDiff
	stateDiff.NewRoot = update.NewRoot
	stateDiff.BlockNumber = blockNumber
	stateDiff.OldRoot = update.OldRoot
	stateDiff.DeployedContracts = make([]starknetTypes.DeployedContract, len(update.StateDiff.DeployedContracts))
	for i, v := range update.StateDiff.DeployedContracts {
		stateDiff.DeployedContracts[i] = starknetTypes.DeployedContract{
			Address:      v.Address,
			ContractHash: v.ContractHash,
		}
	}
	stateDiff.StorageDiffs = make(map[string][]starknetTypes.KV)
	for addressDiff, keyVals := range update.StateDiff.StorageDiffs {
		address := addressDiff
		kvs := make([]starknetTypes.KV, 0)
		for _, kv := range keyVals {
			kvs = append(kvs, starknetTypes.KV{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
		stateDiff.StorageDiffs[address] = kvs
	}

	return &stateDiff
}
