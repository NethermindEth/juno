package services

import (
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"strconv"
	"time"
)

type Synchronizer struct {
	// stateManager is the state manager instance.
	stateManager state.StateManager
	// syncManager represents the sync manager
	syncManager *sync.Manager
	// feederClient is the feeder client instance.
	feederClient *feeder.Client
	// stateDiffCollector is the state diff collector instance.
	stateDiffCollector StateDiffCollector
	// latestBlockOnChain is the latest block on chain.
	latestBlockOnChain int64
}

func NewSynchronizer(syncManager *sync.Manager, stateManager state.StateManager, feederClient *feeder.Client,
	stateDiffCollector StateDiffCollector) *Synchronizer {

	return &Synchronizer{
		stateManager:       stateManager,
		syncManager:        syncManager,
		feederClient:       feederClient,
		stateDiffCollector: stateDiffCollector,
	}
}

func (s *Synchronizer) Run() {
	latestBlockInfoFetched := s.syncManager.GetLatestBlockInfoFetched()
	go s.updateLatestBlockOnChain()
	currentBlock := latestBlockInfoFetched
	for {
		if currentBlock == s.latestBlockOnChain {

		}
		err := s.updateBlock(currentBlock)
		if err != nil {
			return
		}
		s.syncManager.StoreLatestBlockInfoFetched(currentBlock)
		currentBlock++

	}

}

func (s *Synchronizer) updateBlock(blockNumber int64) error {
	block, err := s.feederClient.GetBlock("", strconv.FormatInt(blockNumber, 10))
	if err != nil {

		return err
	}

	for _, txn := range block.Transactions {
		err = s.updateTransactions(txn)
		if err != nil {
			iterations := 0
			for {
				time.Sleep(time.Second * 1)
				if err = s.updateTransactions(txn); err == nil {
					break
				}
				if iterations > 20 {
					return err
				}
				iterations++
			}
			if err != nil {
				return err
			}
		}
	}

	blockHash := types.HexToBlockHash(block.BlockHash)
	BlockService.StoreBlock(blockHash, feederBlockToDBBlock(block))
	return nil
}

func (s *Synchronizer) updateTransactions(txn feeder.TxnSpecificInfo) error {

	transactionInfo, err := s.feederClient.GetTransaction(txn.TransactionHash, "")
	if err != nil {
		return err
	}

	transactionHash := types.HexToTransactionHash(transactionInfo.Transaction.TransactionHash)
	TransactionService.StoreTransaction(transactionHash, feederTransactionToDBTransaction(transactionInfo))
	return nil
}

func (s *Synchronizer) updateContractState() {

}

func (s *Synchronizer) updateLatestBlockOnChain() {

}

// feederBlockToDBBlock convert the feeder block to the block stored in the database
func feederBlockToDBBlock(b *feeder.StarknetBlock) *types.Block {
	txnsHash := make([]types.TransactionHash, 0)
	for _, data := range b.Transactions {
		txnsHash = append(txnsHash, types.TransactionHash(types.HexToFelt(data.TransactionHash)))
	}
	status, _ := types.BlockStatusValue[b.Status]
	return &types.Block{
		BlockHash:   types.HexToBlockHash(b.BlockHash),
		BlockNumber: uint64(b.BlockNumber),
		ParentHash:  types.HexToBlockHash(b.ParentBlockHash),
		Status:      status,
		Sequencer:   types.HexToAddress(b.SequencerAddress),
		NewRoot:     types.HexToFelt(b.StateRoot),
		OldRoot:     types.HexToFelt(b.OldStateRoot),
		TimeStamp:   b.Timestamp,
		TxCount:     uint64(len(b.Transactions)),
		TxHashes:    txnsHash,
	}
}

// feederTransactionToDBTransaction convert the feeder TransactionInfo to the transaction stored in DB
func feederTransactionToDBTransaction(info *feeder.TransactionInfo) types.IsTransaction {
	calldata := make([]types.Felt, 0)
	for _, data := range info.Transaction.Calldata {
		calldata = append(calldata, types.HexToFelt(data))
	}

	if info.Transaction.Type == "INVOKE" {
		signature := make([]types.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, types.HexToFelt(data))
		}
		return &types.TransactionInvoke{
			Hash:               types.HexToTransactionHash(info.Transaction.TransactionHash),
			ContractAddress:    types.HexToAddress(info.Transaction.ContractAddress),
			EntryPointSelector: types.HexToFelt(info.Transaction.EntryPointSelector),
			CallData:           calldata,
			Signature:          signature,
			MaxFee:             types.Felt{},
		}
	}

	// Is a DEPLOY Transaction
	return &types.TransactionDeploy{
		Hash:                types.HexToTransactionHash(info.Transaction.TransactionHash),
		ContractAddress:     types.HexToAddress(info.Transaction.ContractAddress),
		ConstructorCallData: calldata,
	}
}
