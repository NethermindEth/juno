package starknet

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/cairovm"
	"go.uber.org/zap"

	sync2 "github.com/NethermindEth/juno/internal/sync"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"

	"github.com/NethermindEth/juno/internal/db"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/felt"
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
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, err
	}
	return NewBlockWithTxHashes(b), nil
}

func (s *StarkNetRpc) GetBlockWithTxs(blockId *BlockId) (any, error) {
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, err
	}
	return NewBlockWithTxs(b, s.txnManager)
}

func (s *StarkNetRpc) GetStateUpdate(blockId *BlockId) (any, error) {
	if blockId == nil {
		return nil, nil
	}
	switch blockId.idType {
	case blockIdHash:
		hash, _ := blockId.hash()
		return s.synchronizer.GetStateDiffFromHash(hash.Hex()), nil
	case blockIdNumber:
		number, _ := blockId.number()
		return s.synchronizer.GetStateDiff(int64(number)), nil
	default:
		return nil, NewInvalidBlockId()
	}
}

func (s *StarkNetRpc) GetStorageAt(address string, key string, blockId *BlockId) (any, error) {
	if !isStorageKey(key) {
		return nil, NewInvalidStorageKey()
	}
	keyF := new(felt.Felt).SetHex(key)
	if !isFelt(address) {
		return nil, NewContractNotFound()
	}
	addressF := new(felt.Felt).SetHex(address)
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, err
	}
	_state := state.New(s.stateManager, b.NewRoot)
	value, err := _state.GetSlot(addressF, keyF)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewContractNotFound()
		}
        s.logger.Errorw(err.Error(), "function", "GetStorageAt")
        return nil, errors.New("unexpected error")
	}
	return value.Hex0x(), nil
}

func (s *StarkNetRpc) GetTransactionByHash(transactionHash string) (any, error) {
	if !isFelt(transactionHash) {
		return nil, NewInvalidTxnHash()
	}
	txHash := new(felt.Felt).SetHex(transactionHash)
	tx, err := s.txnManager.GetTransaction(txHash)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewInvalidTxnHash()
		}
        s.logger.Errorw(err.Error(), "function", "GetTransactionByHash")
        return nil, errors.New("unexpected error")
	}
	return NewTxn(tx)
}

func (s *StarkNetRpc) GetTransactionByBlockIdAndIndex(blockId *BlockId, index uint64) (any, error) {
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, err
	}
	if index >= b.TxCount {
		return nil, NewInvalidTxnIndex()
	}
	txHash := b.TxHashes[index]
	tx, err := s.txnManager.GetTransaction(txHash)
	if err != nil {
        s.logger.Errorw(err.Error(), "function", "GetTransactionByBlockIdAndIndex")
        return nil, errors.New("unexpected error")
	}
	return NewTxn(tx)
}

func (s *StarkNetRpc) GetTransactionReceipt(transactionHash string) (any, error) {
	if !isFelt(transactionHash) {
		return nil, NewInvalidTxnHash()
	}
	txHash := new(felt.Felt).SetHex(transactionHash)
	_receipt, err := s.txnManager.GetReceipt(txHash)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewInvalidTxnHash()
		}
		s.logger.Errorw(err.Error(), "function", "GetTransactionReceipt")
		return nil, errors.New("unexpected error")
	}
	return NewReceipt(_receipt)
}

func (s *StarkNetRpc) GetClass(classHash string) (any, error) {
	if !isFelt(classHash) {
		return nil, NewInvalidContractClassHash()
	}
	_ = new(felt.Felt).SetHex(classHash)
	_, latestBlockHash := s.synchronizer.LatestBlockSynced()
	latestBlock, err := s.blockManager.GetBlockByHash(latestBlockHash)
	if err != nil {
        if errors.Is(err, db.ErrNotFound) {
            return nil, NewInvalidContractClassHash()
        }
        s.logger.Errorw(err.Error(), "function", "GetClass")
		return nil, errors.New("unexpected error")
	}
	_ = state.New(s.stateManager, latestBlock.NewRoot)
	// TODO: implement class service
	return nil, errors.New("unimplemented")
}

func (s *StarkNetRpc) GetClassHashAt(blockId *BlockId, address string) (any, error) {
	if !isFelt(address) {
		return nil, NewContractNotFound()
	}
	addressF := new(felt.Felt).SetHex(address)
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, NewInvalidBlockId()
	}
	_state := state.New(s.stateManager, b.NewRoot)
	classHash, err := _state.GetClassHash(addressF)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewContractNotFound()
		}
        s.logger.Errorw(err.Error(), "function", "GetClassHashAt")
        return nil, errors.New("unexpected error")
	}
	if classHash.IsZero() {
		return nil, NewContractNotFound()
	}
	return classHash.Hex0x(), nil
}

func (s *StarkNetRpc) GetBlockTransactionCount(blockId *BlockId) (any, error) {
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, err
	}
	return b.TxCount, nil
}

func (s *StarkNetRpc) Call(blockId *BlockId, request *FunctionCall) (any, error) {
	var (
		callData           = make([]*felt.Felt, len(request.Calldata))
		contractAddress    *felt.Felt
		entryPointSelector *felt.Felt
	)
	for i, data := range request.Calldata {
		if !isFelt(data) {
			return nil, NewInvalidCallData()
		}
		callData[i] = new(felt.Felt).SetHex(data)
	}
	if !isFelt(request.ContractAddress) {
		return nil, NewContractNotFound()
	}
	contractAddress = new(felt.Felt).SetHex(request.ContractAddress)
	if !isFelt(request.EntryPointSelector) {
		return nil, NewInvalidMessageSelector()
	}
	entryPointSelector = new(felt.Felt).SetHex(request.EntryPointSelector)
	b, err := getBlockById(blockId, s.blockManager)
	if err != nil {
		return nil, err
	}
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
		return nil, errors.New("unexpected error")
	}
	return out, nil
}

func (s *StarkNetRpc) EstimateFee(blockId *BlockId, request *InvokeTxn) (any, error) {
	// TODO: implement
	return nil, errors.New("not implemented")
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
