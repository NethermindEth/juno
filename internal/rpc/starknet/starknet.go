package starknet

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/cairovm"

	sync2 "github.com/NethermindEth/juno/internal/sync"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/jsonrpc"
	"github.com/NethermindEth/juno/pkg/log"
	"github.com/NethermindEth/juno/pkg/state"
)

type StarkNetRpc struct {
	stateManager state.StateManager
	blockManager *block.Manager
	txnManager   *transaction.Manager
	synchronizer *sync2.Synchronizer
	vm           *cairovm.VirtualMachine
	logger       log.Logger
}

func New(stateManager state.StateManager, blockManager *block.Manager, txnManager *transaction.Manager,
	synchronizer *sync2.Synchronizer, vm *cairovm.VirtualMachine, logger log.Logger,
) *StarkNetRpc {
	return &StarkNetRpc{
		stateManager: stateManager,
		blockManager: blockManager,
		txnManager:   txnManager,
		synchronizer: synchronizer,
		vm:           vm,
		logger:       logger,
	}
}

func (s *StarkNetRpc) GetBlockWithTxHashes(blockId *BlockId) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	return NewBlockWithTxHashes(b), nil
}

func (s *StarkNetRpc) GetBlockWithTxs(blockId *BlockId) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	return NewBlockWithTxs(b, s.txnManager)
}

func (s *StarkNetRpc) GetStateUpdate(blockId *BlockId) (any, error) {
	if blockId == nil {
		return nil, nil
	}
	block, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	diff, err := s.synchronizer.GetStateDiff(block.BlockHash)
	if err != nil {
		return nil, err
	}
	return NewStateUpdate(diff), nil
}

func (s *StarkNetRpc) GetStorageAt(address *RpcFelt, key *StorageKey, blockId *BlockId) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	_state := state.New(s.stateManager, b.NewRoot)
	value, err := _state.GetSlot(address.Felt(), key.Felt())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ContractNotFound
		}
		s.logger.Errorw(err.Error(), "function", "GetStorageAt")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	return value.Hex0x(), nil
}

func (s *StarkNetRpc) GetTransactionByHash(transactionHash *RpcFelt) (any, error) {
	tx, err := s.txnManager.GetTransaction(transactionHash.Felt())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, InvalidTxnHash
		}
		s.logger.Errorw(err.Error(), "function", "GetTransactionByHash")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	return NewTxn(tx)
}

func (s *StarkNetRpc) GetTransactionByBlockIdAndIndex(blockId *BlockId, index *uint64) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	if index == nil || *index >= b.TxCount {
		return nil, InvalidTxnIndex
	}
	txHash := b.TxHashes[*index]
	tx, err := s.txnManager.GetTransaction(txHash)
	if err != nil {
		s.logger.Errorw(err.Error(), "function", "GetTransactionByBlockIdAndIndex")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	return NewTxn(tx)
}

func (s *StarkNetRpc) GetTransactionReceipt(transactionHash *RpcFelt) (any, error) {
	receipt, err := s.txnManager.GetReceipt(transactionHash.Felt())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, InvalidTxnHash
		}
		s.logger.Errorw(err.Error(), "function", "GetTransactionReceipt")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	return NewReceipt(receipt)
}

func (s *StarkNetRpc) GetClass(classHash *RpcFelt) (any, error) {
	_, latestBlockHash := s.synchronizer.LatestBlockSynced()
	latestBlock, err := s.blockManager.GetBlockByHash(latestBlockHash)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, InvalidContractClassHash
		}
		s.logger.Errorw(err.Error(), "function", "GetClass")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	_ = state.New(s.stateManager, latestBlock.NewRoot)
	// TODO: implement class service
	return nil, jsonrpc.NewInternalError("not implemented")
}

func (s *StarkNetRpc) GetClassHashAt(blockId *BlockId, address *RpcFelt) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	_state := state.New(s.stateManager, b.NewRoot)
	classHash, err := _state.GetClassHash(address.Felt())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ContractNotFound
		}
		s.logger.Errorw(err.Error(), "function", "GetClassHashAt")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	if classHash.IsZero() {
		return nil, ContractNotFound
	}
	return classHash.Hex0x(), nil
}

func (s *StarkNetRpc) GetBlockTransactionCount(blockId *BlockId) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	return b.TxCount, nil
}

func (s *StarkNetRpc) Call(blockId *BlockId, request *FunctionCall) (any, error) {
	b, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}
	if request == nil {
		return nil, InvalidCallData
	}
	var (
		callData           = make([]*felt.Felt, len(request.Calldata))
		contractAddress    *felt.Felt
		entryPointSelector *felt.Felt
	)
	for i, data := range request.Calldata {
		if !isFelt(data) {
			return nil, InvalidCallData
		}
		callData[i] = new(felt.Felt).SetHex(data)
	}
	if !isFelt(request.ContractAddress) {
		return nil, ContractNotFound
	}
	contractAddress = new(felt.Felt).SetHex(request.ContractAddress)
	if !isFelt(request.EntryPointSelector) {
		return nil, InvalidMessageSelector
	}
	entryPointSelector = new(felt.Felt).SetHex(request.EntryPointSelector)
	_state := state.New(s.stateManager, b.NewRoot)
	out, err := s.vm.Call(
		context.Background(),
		_state,
		callData,
		new(felt.Felt),
		contractAddress,
		entryPointSelector,
		b.Sequencer,
	)
	if err != nil {
		s.logger.Errorw(err.Error(), "function", "Call")
		return nil, jsonrpc.NewInternalError(err.Error())
	}
	_out := make([]string, len(out))
	for i, v := range out {
		_out[i] = v.Hex0x()
	}
	return _out, nil
}

func (s *StarkNetRpc) EstimateFee(blockId *BlockId, request *InvokeTxn) (any, error) {
	// TODO: implement
	return nil, jsonrpc.NewInternalError("not implemented")
}

func (s *StarkNetRpc) BlockNumber() (any, error) {
	bNumber, _ := s.synchronizer.LatestBlockSynced()
	return bNumber, nil
}

func (s *StarkNetRpc) BlockHashAndNumber() (any, error) {
	type Response struct {
		BlockHash   string `json:"block_hash"`
		BlockNumber int64  `json:"block_number"`
	}

	bNumber, bHash := s.synchronizer.LatestBlockSynced()

	return Response{
		BlockHash:   bHash.Hex0x(),
		BlockNumber: bNumber,
	}, nil
}

func (s *StarkNetRpc) ChainId() (any, error) {
	chainId := s.synchronizer.ChainID()
	return fmt.Sprintf("%x", chainId), nil
}

func (s *StarkNetRpc) PendingTransactions() (any, error) {
	return s.synchronizer.GetPendingBlock().Transactions, nil
}

func (s *StarkNetRpc) ProtocolVersion() (any, error) {
	return "0", nil
}

func (s *StarkNetRpc) Syncing() (any, error) {
	if s.synchronizer.Running {
		return s.synchronizer.Status(), nil
	}
	return false, nil
}

func (s *StarkNetRpc) HealthCheck() (any, error) {
	// notest
	return Status{Available: true}, nil
}
