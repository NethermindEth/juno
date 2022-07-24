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

func (s *Synchronizer) UpdateBlock(blockNumber int64) (*feeder.StarknetBlock, error) {
	block, err := s.feederClient.GetBlock("", strconv.FormatInt(blockNumber, 10))
	if err != nil {
		return nil, err
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
					return nil, err
				}
				iterations++
			}
			if err != nil {
				return nil, err
			}
		}
	}

	blockHash := new(felt.Felt).SetHex(block.BlockHash)
	err = BlockService.StoreBlock(blockHash, feederBlockToDBBlock(block))
	if err != nil {
		return nil, err
	}
	return block, nil
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

	if info.Transaction.Type == "INVOKE_FUNCTION" {
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

	if info.Transaction.Type == "DECLARE" {

		signature := make([]*felt.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, new(felt.Felt).SetHex(data))
		}
		return &types.TransactionDeclare{
			Hash:          new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ClassHash:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			SenderAddress: new(felt.Felt).SetHex(info.Transaction.SenderAddress),
			MaxFee:        new(felt.Felt).SetHex(info.Transaction.MaxFee),
			Signature:     signature,
			Nonce:         new(felt.Felt).SetHex(info.Transaction.Nonce),
			Version:       new(felt.Felt).SetHex(info.Transaction.Version),
		}
	}

	// Is a DEPLOY Transaction
	return &types.TransactionDeploy{
		Hash:                new(felt.Felt).SetHex(info.Transaction.TransactionHash),
		ContractAddress:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
		ConstructorCallData: calldata,
	}
}
