package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/log"
)

// Echo replies with the same message.
func (HandlerRPC) Echo(c context.Context, message string) (string, error) {
	return message, nil
}

// EchoErr replies with the same message as an error.
func (HandlerRPC) EchoErr(c context.Context, message string) (string, error) {
	return "", fmt.Errorf("%s", message)
}

// StarknetCall represents the handler of "starknet_call" rpc call.
func (HandlerRPC) StarknetCall(
	c context.Context, request FunctionCall, blockHash BlockHashOrTag,
) (ResultCall, error) {
	return []string{"Response", "of", "starknet_call"}, nil
}

func getBlockByTag(ctx context.Context, blockTag BlockTag, scope RequestedScope) (*BlockResponse, error) {
	// notest
	// TODO: Implement get block by tag
	return &BlockResponse{}, nil
}

func getBlockByHash(ctx context.Context, blockHash types.BlockHash, scope RequestedScope) (*BlockResponse, error) {
	log.Default.With("blockHash", blockHash, "scope", scope).Info("StarknetGetBlockByHash")
	dbBlock := services.BlockService.GetBlockByHash(blockHash)
	if dbBlock == nil {
		// notest
		// TODO: Send custom error for not found. Maybe sent InvalidBlockHash?
		return nil, errors.New("block not found")
	}
	response := NewBlockResponse(dbBlock, scope)
	return response, nil
}

func getBlockByHashOrTag(ctx context.Context, blockHashOrTag BlockHashOrTag, scope RequestedScope) (*BlockResponse, error) {
	if blockHashOrTag.Hash != nil {
		return getBlockByHash(ctx, *blockHashOrTag.Hash, scope)
	}
	// notest
	if blockHashOrTag.Tag != nil {
		return getBlockByTag(ctx, *blockHashOrTag.Tag, scope)
	}
	return nil, ErrInvalidRequest()
}

// StarknetGetBlockByHash represent the handler for getting a block by
// its hash.
func (HandlerRPC) StarknetGetBlockByHash(ctx context.Context, blockHashOrTag BlockHashOrTag) (*BlockResponse, error) {
	// notest
	return getBlockByHashOrTag(ctx, blockHashOrTag, ScopeTxnHash)
}

// StarknetGetBlockByHashOpt represent the handler for getting a block
// by its hash.
func (HandlerRPC) StarknetGetBlockByHashOpt(ctx context.Context, blockHashOrTag BlockHashOrTag, scope RequestedScope) (*BlockResponse, error) {
	return getBlockByHashOrTag(ctx, blockHashOrTag, scope)
}

func getBlockByNumber(ctx context.Context, blockNumber uint64, scope RequestedScope) (*BlockResponse, error) {
	log.Default.With("blockNumber", blockNumber, "scope", scope).Info("StarknetGetBlockNyNumber")
	dbBlock := services.BlockService.GetBlockByNumber(blockNumber)
	if dbBlock == nil {
		// notest
		return nil, errors.New("block not found")
	}
	response := NewBlockResponse(dbBlock, scope)
	return response, nil
}

func getBlockByNumberOrTag(ctx context.Context, blockNumberOrTag BlockNumberOrTag, scope RequestedScope) (*BlockResponse, error) {
	if number := blockNumberOrTag.Number; number != nil {
		return getBlockByNumber(ctx, *number, scope)
	}
	// notest
	if tag := blockNumberOrTag.Tag; tag != nil {
		return getBlockByTag(ctx, *tag, scope)
	}
	// TODO: Send bad request error
	return nil, errors.New("bad request")
}

// type bNumber string `json:"int,int,omitempty"`

// StarknetGetBlockByNumber represent the handler for getting a block by
// its number.
func (HandlerRPC) StarknetGetBlockByNumber(ctx context.Context, blockNumberOrTag BlockNumberOrTag) (*BlockResponse, error) {
	// notest
	return getBlockByNumberOrTag(ctx, blockNumberOrTag, ScopeTxnHash)
}

// StarknetGetBlockByNumberOpt represent the handler for getting a block
// by its number.
func (HandlerRPC) StarknetGetBlockByNumberOpt(ctx context.Context, blockNumberOrTag BlockNumberOrTag, scope RequestedScope) (*BlockResponse, error) {
	return getBlockByNumberOrTag(ctx, blockNumberOrTag, scope)
}

// StarknetGetBlockTransactionCountByHash represent the handler for
// getting block transaction count by the blocks hash.
func (HandlerRPC) StarknetGetBlockTransactionCountByHash(
	c context.Context, blockHash BlockHashOrTag,
) (BlockTransactionCount, error) {
	return BlockTransactionCount{}, nil
}

// StarknetGetBlockTransactionCountByNumber Get the number of
// transactions in a block given a block number (height).
func (HandlerRPC) StarknetGetBlockTransactionCountByNumber(
	c context.Context, blockNumber interface{},
) (BlockTransactionCount, error) {
	return BlockTransactionCount{}, nil
}

// StarknetGetStateUpdateByHash represent the handler for getting the
// information about the result of executing the requested block.
func (HandlerRPC) StarknetGetStateUpdateByHash(
	c context.Context, blockHash BlockHashOrTag,
) (StateUpdate, error) {
	return StateUpdate{}, nil
}

// StarknetGetStorageAt Get the value of the storage at the given
// address and key.
func (HandlerRPC) StarknetGetStorageAt(
	c context.Context,
	contractAddress Address,
	key Felt,
	blockHash BlockHashOrTag,
) (Felt, error) {
	if hash := blockHash.Hash; hash != nil {
		block := services.BlockService.GetBlockByHash(*blockHash.Hash)
		if block == nil {
			// notest
			return "", fmt.Errorf("block not found")
		}
		storage := services.StateService.GetStorage(string(contractAddress), block.BlockNumber)
		if storage == nil {
			// notest
			return "", fmt.Errorf("storage not found")
		}
		return Felt(storage.Storage[string(key)]), nil
	}
	// notest
	if tag := blockHash.Tag; tag != nil {
		// TODO: Get by tag
		return "", fmt.Errorf("unimplmented search by block tag")
	}
	return "", fmt.Errorf("invalid block hash or tag")
}

// StarknetGetTransactionByHash Get the details and status of a
// submitted transaction.
func (HandlerRPC) StarknetGetTransactionByHash(
	c context.Context, transactionHash types.TransactionHash,
) (*Txn, error) {
	tx := services.TransactionService.GetTransaction(transactionHash)
	if tx == nil {
		// notest
		// TODO: return not found error
		return &Txn{}, nil
	}
	return NewTxn(tx), nil
}

// StarknetGetTransactionByBlockHashAndIndex Get the details of the
// transaction given by the identified block and index in that block. If
// no transaction is found, a null value is returned.
func (HandlerRPC) StarknetGetTransactionByBlockHashAndIndex(c context.Context, blockHashOrTag BlockHashOrTag, index int) (*Txn, error) {
	if blockHash := blockHashOrTag.Hash; blockHash != nil {
		block := services.BlockService.GetBlockByHash(*blockHash)
		if index < 0 || len(block.TxHashes) <= index {
			// notest
			return nil, fmt.Errorf("invalid index %d", index)
		}
		txHash := block.TxHashes[index]
		txn := services.TransactionService.GetTransaction(txHash)
		return NewTxn(txn), nil
	}
	// notest
	if tag := blockHashOrTag.Tag; tag != nil {
		// TODO: search block by tag
		return &Txn{}, nil
	}
	// TODO: return invalid param error
	return nil, errors.New("invalid blockHashOrtTag param")
}

// StarknetGetTransactionByBlockNumberAndIndex Get the details of the
// transaction given by the identified block and index in that block. If
// no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockNumberAndIndex(ctx context.Context, blockNumberOrTag BlockNumberOrTag, index int) (*Txn, error) {
	if blockNumber := blockNumberOrTag.Number; blockNumber != nil {
		block := services.BlockService.GetBlockByNumber(*blockNumber)
		if index < 0 || len(block.TxHashes) <= index {
			// notest
			return nil, fmt.Errorf("invalid index %d", index)
		}
		txHash := block.TxHashes[index]
		txn := services.TransactionService.GetTransaction(txHash)
		return NewTxn(txn), nil
	}
	// notest
	if tag := blockNumberOrTag.Tag; tag != nil {
		// TODO: search block by tag
		return &Txn{}, nil
	}
	// TODO: return invalid param error
	return nil, errors.New("invalid blockHashOrtTag param")
}

// StarknetGetTransactionReceipt Get the transaction receipt by the
// transaction hash.
func (HandlerRPC) StarknetGetTransactionReceipt(
	c context.Context, transactionHash TxnHash,
) (TxnReceipt, error) {
	return TxnReceipt{}, nil
}

// StarknetGetCode Get the code of a specific contract
func (HandlerRPC) StarknetGetCode(
	c context.Context, contractAddress types.Address,
) (CodeResult, error) {
	abi := services.AbiService.GetAbi(contractAddress.Hex())
	if abi == nil {
		// notest
		return CodeResult{}, fmt.Errorf("abi not found")
	}
	code := services.StateService.GetBinaryCode(contractAddress.Bytes())
	if code == nil {
		// notest
		return CodeResult{}, fmt.Errorf("code not found")
	}
	marshalledAbi, err := json.Marshal(abi)
	if err != nil {
		return CodeResult{}, err
	}
	bytecode := make([]Felt, len(code.Code))
	for i, b := range code.Code {
		bytecode[i] = Felt(types.BytesToFelt(b).Hex())
	}
	return CodeResult{Abi: string(marshalledAbi), Bytecode: bytecode}, nil
}

// StarknetBlockNumber Get the most recent accepted block number
func (HandlerRPC) StarknetBlockNumber(c context.Context) (BlockNumber, error) {
	return 0, nil
}

// StarknetChainId Return the currently configured StarkNet chain id
func (HandlerRPC) StarknetChainId(c context.Context) (ChainID, error) {
	return "Here the ChainID", nil
}

// StarknetPendingTransactions Returns the transactions in the
// transaction pool, recognized by this sequencer.
func (HandlerRPC) StarknetPendingTransactions(
	c context.Context,
) ([]Txn, error) {
	return nil, nil
}

// StarknetProtocolVersion Returns the current starknet protocol version
// identifier, as supported by this sequencer.
func (HandlerRPC) StarknetProtocolVersion(
	c context.Context,
) (ProtocolVersion, error) {
	return "Here the Protocol Version", nil
}

// StarknetSyncing Returns an object about the sync status, or false if
// the node is not syncing.
func (HandlerRPC) StarknetSyncing(
	c context.Context,
) (SyncStatus, error) {
	return SyncStatus{}, nil
}

// StarknetGetEvents Returns all event objects matching the conditions
// in the provided filter.
func (HandlerRPC) StarknetGetEvents(
	c context.Context, r EventRequest,
) (EventResponse, error) {
	return EventResponse{}, nil
}
