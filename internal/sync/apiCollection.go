package sync

import (
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/log"
	"go.uber.org/zap"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
)

type apiCollector struct {
	logger *zap.SugaredLogger
	// manager is the Sync manager
	manager *sync.Manager
	// feeder is the client that provides the Connection to the feeder gateway.
	client *feeder.Client
	// buffer represent the channel of StateDiff collected
	buffer chan *CollectorDiff

	// latestBlock is the last block of StarkNet
	latestBlock *feeder.StarknetBlock
	// pendingBlock is the pending block of StarkNet
	pendingBlock *feeder.StarknetBlock

	quit chan struct{}
}

// NewApiCollector create a new api Collector
// notest
func NewApiCollector(manager *sync.Manager, feeder *feeder.Client) *apiCollector {
	collector := &apiCollector{
		client:  feeder,
		manager: manager,
		quit:    make(chan struct{}),
	}
	collector.logger = log.Logger.Named("apiCollector")
	collector.buffer = make(chan *CollectorDiff, 10)
	go collector.updateLatestBlockOnChain()
	return collector
}

// Run start to store StateDiff locally
func (a *apiCollector) Run() {
	a.logger.Info("Collector Started")
	// start the buffer updater
	latestStateDiffSynced := a.manager.GetLatestBlockSync()
	for {
		select {
		case <-a.quit:
			close(a.buffer)
			return
		default:
			if a.latestBlock == nil {
				time.Sleep(time.Second * 3)
				continue
			}
			if latestStateDiffSynced >= int64(a.latestBlock.BlockNumber) {
				time.Sleep(time.Second * 3)
				continue
			}
			var update *feeder.StateUpdateResponse
			var err error
			update, err = a.client.GetStateUpdate("", strconv.FormatInt(latestStateDiffSynced, 10))
			if err != nil {
				a.logger.With("Error", err, "Block Number", latestStateDiffSynced).Info("Couldn't get state update")
				continue
			}
			a.buffer <- fetchContractCode(stateUpdateResponseToStateDiff(*update, latestStateDiffSynced), a.client, a.logger)
			a.logger.With("BlockNumber", latestStateDiffSynced).Info("StateUpdate collected")
			latestStateDiffSynced += 1
		}
	}
}

func (a *apiCollector) Close() {
	close(a.quit)
}

// GetChannel returns the channel of StateDiffs
func (a *apiCollector) GetChannel() chan *CollectorDiff {
	return a.buffer
}

// updateLatestBlockOnChain update the latest block on chain
func (a *apiCollector) updateLatestBlockOnChain() {
	a.fetchLatestBlockOnChain()
	for {
		time.Sleep(time.Minute)
		a.fetchLatestBlockOnChain()
	}
}

// fetchLatestBlockOnChain fetch the latest block on chain
func (a *apiCollector) fetchLatestBlockOnChain() {
	pendingBlock, err := a.client.GetBlock("", "pending")
	if err != nil {
		return
	}
	if a.pendingBlock != nil && pendingBlock.ParentBlockHash == a.pendingBlock.ParentBlockHash {
		return
	}

	parentBlock, err := a.client.GetBlock(pendingBlock.ParentBlockHash, "")
	if err != nil {
		return
	}

	a.latestBlock = parentBlock
	a.pendingBlock = pendingBlock
}

func (a *apiCollector) LatestBlock() *feeder.StarknetBlock {
	return a.latestBlock
}

func (a *apiCollector) PendingBlock() *feeder.StarknetBlock {
	return a.pendingBlock
}

// stateUpdateResponseToStateDiff convert the input feeder.StateUpdateResponse to StateDiff
func stateUpdateResponseToStateDiff(update feeder.StateUpdateResponse, blockNumber int64) *types.StateUpdate {
	var stateDiff types.StateUpdate
	stateDiff.NewRoot = new(felt.Felt).SetHex(update.NewRoot)
	stateDiff.BlockNumber = blockNumber
	stateDiff.BlockHash = new(felt.Felt).SetHex(update.BlockHash)
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
		ca := new(felt.Felt).SetHex(contractAddress)
		// Create felt and convert to string for consistency
		stateDiff.StorageDiff[*ca] = kvs
	}

	return &stateDiff
}
