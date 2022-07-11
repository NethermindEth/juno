package starknet

import (
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/types"
)

func ApiLoadStateDiffs(startStarknetBlock uint64, feederClient feeder.Client, stateDiffsChan chan *types.StateUpdate, errChan chan error) {
	defer close(stateDiffsChan)
	for blockNum := startStarknetBlock; ; blockNum++ {
		select {
		case err := <-errChan:
			log.Default.With("error", err).Error("unexpected error")
			return
		default:
			log.Default.With("blockNumber", blockNum).Info("requesting state update from feeder gateway")
			feederStateUpdate, err := feederClient.GetStateUpdate("", strconv.FormatUint(blockNum, 10))
			if err != nil {
				// Assume we are synced or there was an error
				log.Default.Info("no update from feeder gateway, pausing for one minute")
				time.Sleep(time.Minute)
				continue
			}
			stateDiffsChan <- feederStateUpdateToStateDiff(*feederStateUpdate)
		}
	}
}

func feederStateUpdateToStateDiff(update feeder.StateUpdateResponse) *types.StateUpdate {
	var stateUpdate types.StateUpdate
	stateUpdate.StateDiff.DeployedContracts = make([]types.DeployedContract, len(update.StateDiff.DeployedContracts))
	for i, v := range update.StateDiff.DeployedContracts {
		stateUpdate.StateDiff.DeployedContracts[i] = types.DeployedContract{
			Address: v.Address,
			Hash:    v.ContractHash,
		}
	}
	stateUpdate.StateDiff.StorageDiffs = make(map[string][]types.MemoryCell)
	for addressDiff, keyVals := range update.StateDiff.StorageDiffs {
		address := addressDiff
		kvs := make([]types.MemoryCell, 0)
		for _, kv := range keyVals {
			kvs = append(kvs, types.MemoryCell{
				Address: kv.Key,
				Value:   kv.Value,
			})
		}
		stateUpdate.StateDiff.StorageDiffs[address] = kvs
	}

	return &stateUpdate
}
