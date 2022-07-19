package services

import (
	"strconv"
	"time"

	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
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

	pendingBlock *feeder.StarknetBlock
}

func NewSynchronizer(syncManager *sync.Manager, stateManager state.StateManager, feederClient *feeder.Client,
	stateDiffCollector StateDiffCollector,
) *Synchronizer {
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

	blockHash := new(felt.Felt).SetHex(block.BlockHash)
	err = BlockService.StoreBlock(blockHash, feederBlockToDBBlock(block))
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) updateTransactions(txn feeder.TxnSpecificInfo) error {
	transactionInfo, err := s.feederClient.GetTransaction(txn.TransactionHash, "")
	if err != nil {
		return err
	}

	transactionHash := new(felt.Felt).SetHex(transactionInfo.Transaction.TransactionHash)
	// transactionHash := types.HexToTransactionHash(transactionInfo.Transaction.TransactionHash)
	err = TransactionService.StoreTransaction(transactionHash, feederTransactionToDBTransaction(transactionInfo))
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) updateLatestBlockOnChain() {
	for {
		time.Sleep(time.Second * 1)
		s.latestBlockOnChain = s.stateDiffCollector.GetLatestBlockOnChain()
	}
}

// feederBlockToDBBlock convert the feeder block to the block stored in the database
func feederBlockToDBBlock(b *feeder.StarknetBlock) *types.Block {
	txnsHash := make([]*felt.Felt, 0)
	for _, data := range b.Transactions {
		txnsHash = append(txnsHash, new(felt.Felt).SetHex(data.TransactionHash))
	}
	status, _ := types.BlockStatusValue[b.Status]
	return &types.Block{
		BlockHash:   new(felt.Felt).SetHex(b.BlockHash),
		BlockNumber: uint64(b.BlockNumber),
		ParentHash:  new(felt.Felt).SetHex(b.ParentBlockHash),
		Status:      status,
		Sequencer:   new(felt.Felt).SetHex(b.SequencerAddress),
		NewRoot:     new(felt.Felt).SetHex(b.StateRoot),
		OldRoot:     new(felt.Felt).SetHex(b.OldStateRoot),
		TimeStamp:   b.Timestamp,
		TxCount:     uint64(len(b.Transactions)),
		TxHashes:    txnsHash,
	}
}

// feederTransactionToDBTransaction convert the feeder TransactionInfo to the transaction stored in DB
func feederTransactionToDBTransaction(info *feeder.TransactionInfo) types.IsTransaction {
	calldata := make([]*felt.Felt, 0)
	for _, data := range info.Transaction.Calldata {
		calldata = append(calldata, new(felt.Felt).SetHex(data))
	}

	if info.Transaction.Type == "INVOKE" {
		signature := make([]*felt.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, new(felt.Felt).SetHex(data))
		}
		return &types.TransactionInvoke{
			Hash:               new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ContractAddress:    new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			EntryPointSelector: new(felt.Felt).SetHex(info.Transaction.EntryPointSelector),
			CallData:           calldata,
			Signature:          signature,
			MaxFee:             new(felt.Felt),
		}
	}

	// Is a DEPLOY Transaction
	return &types.TransactionDeploy{
		Hash:                new(felt.Felt).SetHex(info.Transaction.TransactionHash),
		ContractAddress:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
		ConstructorCallData: calldata,
	}
}
