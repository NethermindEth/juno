package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"strconv"
)

var APICollector *apiCollector

type apiCollector struct {
	// feeder is the client that provides the Connection to the feeder gateway.
	client *feeder.Client
	// manager is the Sync manager
	manager *sync.Manager
	// close is the channel that will be used to close the collector.
	close   chan bool
	chainID int
	service
}

func NewApiCollector(manager *sync.Manager, feeder *feeder.Client, chainID int) {
	APICollector = &apiCollector{
		client:  feeder,
		manager: manager,
		chainID: chainID,
		close:   make(chan bool),
	}
	APICollector.logger = log.Default.Named("apiCollector")
}

// Run start to store StateDiff locally
func (a *apiCollector) Run() error {

	latestStateDiffSynced := a.manager.GetLatestStateDiffSynced()
	for {
		select {
		case <-a.close:
			return nil
		default:
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

			// get StateDiff from response
			stateDiff := stateUpdateResponseToStateDiff(*update, latestStateDiffSynced)

			a.manager.StoreStateDiff(stateDiff)

			a.logger.With("Block Number", stateDiff.BlockNumber).Info("Saved State Diff for block")

			//a.manager.(latestStateDiffSynced)

			latestStateDiffSynced += 1
		}
	}
}
func (a *apiCollector) Close(ctx context.Context) {
	a.close <- true
	// TODO: close context
}

func (a *apiCollector) CreateIterator() StateDiffIterator {
	latestBlockSync := a.manager.GetLatestBlockSync()
	return &APIIterator{currentBlockNumber: latestBlockSync, manager: a.manager}

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
