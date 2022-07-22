package starknet

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/services"

	"github.com/NethermindEth/juno/internal/db"

	"github.com/NethermindEth/juno/pkg/felt"

	"github.com/NethermindEth/juno/pkg/state"
)

type StarkNetRpc struct {
	stateManager state.StateManager
}

func New(stateManager state.StateManager) *StarkNetRpc {
	return &StarkNetRpc{stateManager: stateManager}
}

type GetBlockWithTxHashesP struct {
	BlockId *BlockId `json:"block_id"`
}

func (s *StarkNetRpc) GetBlockWithTxHashes(ctx context.Context, params *GetBlockWithTxHashesP) (any, error) {
	block, err := getBlockById(params.BlockId)
	if err != nil {
		return nil, err
	}
	return NewBlockWithTxHashes(block), nil
}

type GetBlockWithTxsP struct {
	BlockId *BlockId `json:"block_id"`
}

func (s *StarkNetRpc) GetBlockWithTxs(ctx context.Context, params *GetBlockWithTxsP) (any, error) {
	block, err := getBlockById(params.BlockId)
	if err != nil {
		return nil, err
	}
	return NewBlockWithTxs(block)
}

type GetStateUpdateP struct {
	BlockId *BlockId `json:"block_id"`
}

func (s *StarkNetRpc) GetStateUpdate(ctx context.Context, params *GetStateUpdateP) (any, error) {
	switch params.BlockId.idType {
	case blockIdHash:
		hash, _ := params.BlockId.hash()
		return services.SyncService.GetStateDiffFromHash(hash.Hex()), nil
	case blockIdNumber:
		number, _ := params.BlockId.number()
		return services.SyncService.GetStateDiff(int64(number)), nil
	default:
		// TODO: manage unexpected type
		return nil, nil
	}
}

type GetStorageAtP struct {
	Address string   `json:"address"`
	Key     string   `json:"key"`
	BlockId *BlockId `json:"block_id"`
}

func (s *StarkNetRpc) GetStorageAt(ctx context.Context, params *GetStorageAtP) (any, error) {
	// Parsing Key param
	if !isStorageKey(params.Key) {
		// TODO: the rpc spec does not specify what to do if the key is not a storage key
		return nil, nil
	}
	key := new(felt.Felt).SetHex(params.Key)
	// Parsing Address param
	if !isFelt(params.Address) {
		return nil, NewContractNotFound()
	}
	address := new(felt.Felt).SetHex(params.Address)
	// Searching for the block in the database
	block, err := getBlockById(params.BlockId)
	if err != nil {
		return nil, err
	}
	// Building the state of the block
	_state := state.New(s.stateManager, block.NewRoot)
	// Searching for the value of the key in the state
	value, err := _state.GetSlot(address, key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewContractNotFound()
		}
		// TODO: manage unexpected error
	}
	return value, nil
}

type GetTransactionByHashP struct {
	TransactionHash string `json:"transaction_hash"`
}

func (s *StarkNetRpc) GetTransactionByHash(ctx context.Context, params *GetTransactionByHashP) (any, error) {
	// Parsing TransactionHash param
	if !isFelt(params.TransactionHash) {
		return nil, NewInvalidTxnHash()
	}
	txHash := new(felt.Felt).SetHex(params.TransactionHash)
	tx, err := services.TransactionService.GetTransaction(txHash)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewInvalidTxnHash()
		}
		// TODO: manage unexpected error
	}
	return NewTxn(tx)
}

type GetTransactionByBlockIdAndIndexP struct {
	BlockId *BlockId `json:"block_id"`
	Index   uint64   `json:"index"`
}

func (s *StarkNetRpc) GetTransactionByBlockIdAndIndex(ctx context.Context, params *GetTransactionByBlockIdAndIndexP) (any, error) {
	block, err := getBlockById(params.BlockId)
	if err != nil {
		return nil, err
	}
	if params.Index >= block.TxCount {
		return nil, NewInvalidTxnIndex()
	}
	txHash := block.TxHashes[params.Index]
	tx, err := services.TransactionService.GetTransaction(txHash)
	if err != nil {
		// TODO: manage unexpected error
	}
	return NewTxn(tx)
}

type GetTransactionReceiptP struct {
	TxnHash string `json:"transaction_hash"`
}

func (s *StarkNetRpc) GetTransactionReceipt(ctx context.Context, params *GetTransactionReceiptP) (any, error) {
	// Parsing TxnHash param
	if !isFelt(params.TxnHash) {
		return nil, NewInvalidTxnHash()
	}
	txHash := new(felt.Felt).SetHex(params.TxnHash)
	_receipt, err := services.TransactionService.GetReceipt(txHash)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, NewInvalidTxnHash()
		}
		// TODO: manage unexpected error
	}
	receipt, err := NewRecipt(_receipt)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

type GetClassP struct {
	ClassHash string `json:"class_hash"`
}

func (s *StarkNetRpc) GetClass(ctx context.Context, params *GetClassP) (any, error) {
	// Parsing ClassHash param
	if !isFelt(params.ClassHash) {
		return nil, NewInvalidContractClassHash()
	}
	_ = new(felt.Felt).SetHex(params.ClassHash)
	// TODO: implement
	return nil, errors.New("not implemented")
}

type GetClassHashAtP struct {
	BlockId *BlockId `json:"block_id"`
	Address string   `json:"address"`
}

func (s *StarkNetRpc) GetClassHashAt(ctx context.Context, params *GetClassHashAtP) (any, error) {
	if !isFelt(params.Address) {
		return nil, NewContractNotFound()
	}
	_ = new(felt.Felt).SetHex(params.Address)
	// TODO: implement
	return nil, errors.New("not implemented")
}

type GetBlockTransactionCountP struct {
	BlockId *BlockId `json:"block_id"`
}

func (s *StarkNetRpc) GetBlockTransactionCount(ctx context.Context, params *GetBlockTransactionCountP) (any, error) {
	block, err := getBlockById(params.BlockId)
	if err != nil {
		return nil, err
	}
	return block.TxCount, nil
}

type CallP struct {
	Request FunctionCall `json:"request"`
	BlockId *BlockId     `json:"block_id"`
}

func (s *StarkNetRpc) Call(ctx context.Context, param *CallP) (any, error) {
	block, err := getBlockById(param.BlockId)
	if err != nil {
		return nil, err
	}
	_state := state.New(s.stateManager, block.NewRoot)
	out, err := services.VMService.Call(
		context.Background(),
		_state,
		param.Request.Calldata,
		new(felt.Felt),
		param.Request.ContractAddress,
		param.Request.EntryPointSelector,
		block.Sequencer,
	)
	if err != nil {
		// TODO: manage error
		return nil, errors.New("unexpected error")
	}
	return out, nil
}

type EstimateFeeP struct {
	Request *InvokeTxn `json:"request"`
	BlockId *BlockId   `json:"block_id"`
}

func (s *StarkNetRpc) EstimateFee(ctx context.Context, param *EstimateFeeP) (any, error) {
	// TODO: implement
	return nil, errors.New("not implemented")
}

func (s *StarkNetRpc) BlockNumber(ctx context.Context) (any, error) {
	bNumber, _ := services.SyncService.LatestBlockSynced()
	return bNumber, nil
}

func (s *StarkNetRpc) BlockHashAndNumber(ctx context.Context) (any, error) {
	type Response struct {
		BlockHash   string `json:"block_hash"`
		BlockNumber int64  `json:"block_number"`
	}

	bNumber, bHash := services.SyncService.LatestBlockSynced()

	return Response{
		BlockHash:   bHash,
		BlockNumber: bNumber,
	}, nil
}

func (s *StarkNetRpc) ChainId(ctx context.Context) (string, error) {
	chainId := services.SyncService.ChainID()
	return fmt.Sprintf("%x", chainId), nil
}

func (s *StarkNetRpc) PendingTransactions(ctx context.Context) (any, error) {
	return services.SyncService.GetPendingBlock().Transactions, nil
}

func (s *StarkNetRpc) ProtocolVersion(ctx context.Context) (any, error) {
	return "0", nil
}

func (s *StarkNetRpc) Syncing(ctx context.Context) (any, error) {
	if services.SyncService.Running() {
		return services.SyncService.Status(), nil
	}
	return false, nil
}
