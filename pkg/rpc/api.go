package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/types"
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

// StarknetGetBlockByHash represent the handler for getting a block by
// its hash.
func (HandlerRPC) StarknetGetBlockByHash(
	c context.Context, blockHash BlockHashOrTag,
) (BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return BlockResponse{BlockHash: string(blockHash)}, nil
}

// StarknetGetBlockByHashOpt represent the handler for getting a block
// by its hash.
func (HandlerRPC) StarknetGetBlockByHashOpt(
	c context.Context, blockHash BlockHashOrTag, requestedScope RequestedScope,
) (BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return BlockResponse{
		BlockHash:  string(blockHash),
		ParentHash: string(requestedScope),
	}, nil
}

// type bNumber string `json:"int,int,omitempty"`

// StarknetGetBlockByNumber represent the handler for getting a block by
// its number.
func (HandlerRPC) StarknetGetBlockByNumber(
	c context.Context, blockNumber interface{},
) (BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	log.Default.With("Block Number", blockNumber).Info("Calling StarknetGetBlockByNumber")
	return BlockResponse{}, nil
}

// StarknetGetBlockByNumberOpt represent the handler for getting a block
// by its number.
func (HandlerRPC) StarknetGetBlockByNumberOpt(
	c context.Context, blockNumber interface{}, requestedScope RequestedScope,
) (BlockResponse, error) {
	// TODO See if is possible to support overhead without another method
	return BlockResponse{}, nil
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
	block := services.BlockService.GetBlockByHash(types.HexToFelt(string(blockHash)).Bytes())
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

// StarknetGetTransactionByHash Get the details and status of a
// submitted transaction.
func (HandlerRPC) StarknetGetTransactionByHash(
	c context.Context, transactionHash TxnHash,
) (Txn, error) {
	return Txn{}, nil
}

// StarknetGetTransactionByBlockHashAndIndex Get the details of the
// transaction given by the identified block and index in that block. If
// no transaction is found, a null value is returned.
func (HandlerRPC) StarknetGetTransactionByBlockHashAndIndex(
	c context.Context, blockHash BlockHashOrTag, index uint64,
) (Txn, error) {
	return Txn{}, nil
}

// StarknetGetTransactionByBlockNumberAndIndex Get the details of the
// transaction given by the identified block and index in that block. If
// no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockNumberAndIndex(
	c context.Context, blockNumber BlockNumberOrTag, index uint64,
) (Txn, error) {
	return Txn{}, nil
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
	c context.Context, contractAddress Address,
) (CodeResult, error) {
	abi := services.AbiService.GetAbi(string(contractAddress))
	if abi == nil {
		// notest
		return CodeResult{}, fmt.Errorf("abi not found")
	}
	code := services.StateService.GetCode(types.HexToFelt(string(contractAddress)).Bytes())
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
