package starknet

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/cairovm"
	"github.com/NethermindEth/juno/internal/validate"
	"go.uber.org/zap"

	sync2 "github.com/NethermindEth/juno/internal/sync"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"

	"github.com/NethermindEth/juno/internal/db"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/jsonrpc"
	"github.com/NethermindEth/juno/pkg/state"
)

type StarkNetRpc struct {
	stateManager state.StateManager
	blockManager *block.Manager
	txnManager   *transaction.Manager
	synchronizer *sync2.Synchronizer
	vm           *cairovm.VirtualMachine
	logger       *zap.SugaredLogger
}

func New(stateManager state.StateManager, blockManager *block.Manager, txnManager *transaction.Manager,
	synchronizer *sync2.Synchronizer, vm *cairovm.VirtualMachine,
) *StarkNetRpc {
	return &StarkNetRpc{
		stateManager: stateManager,
		blockManager: blockManager,
		txnManager:   txnManager,
		synchronizer: synchronizer,
		vm:           vm,
		logger:       Logger.Named("RPC"),
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
	// TODO: Test this function.
	// notest
	block, err := getBlockById(blockId, s.blockManager, s.logger)
	if err != nil {
		return nil, err
	}

	if request == nil {
		return nil, jsonrpc.ErrInvalidRequest
	}

	newState := state.New(s.stateManager, block.NewRoot)

	if ok := validate.Felt(request.ContractAddress); !ok {
		return nil, ContractNotFound
	}
	contractAddress := new(felt.Felt).SetHex(request.ContractAddress)

	if ok := validate.Felt(request.EntryPointSelector); !ok {
		return nil, InvalidMessageSelector
	}
	entryPointSelector := new(felt.Felt).SetHex(request.EntryPointSelector)

	if ok := validate.Felts(request.Calldata); !ok {
		return nil, InvalidCallData
	}
	calldata := felt.SetStrings(request.Calldata)

	returned, err := s.vm.Call(
		context.Background(),
		newState,
		block.Sequencer,
		contractAddress,
		entryPointSelector,
		calldata,
	)
	if err != nil {
		s.logger.Error("rpc/starknet: call: " + err.Error())
		return nil, jsonrpc.NewInternalError(err.Error())
	}

	response := make([]string, 0, len(returned))
	for _, data := range returned {
		response = append(response, data.Hex0x())
	}

	return response, nil
}

// notest
func (s *StarkNetRpc) EstimateFee(request *InvokeTxnV0, blockId *BlockId) (any, error) {
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
