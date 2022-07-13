package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dbAbi "github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/types"

	. "github.com/NethermindEth/juno/internal/log"
)

var (
	// invalid param
	ErrBlockHashOrTag = errors.New("invalid blockHashOrTag param")
	ErrBlockIndex     = errors.New("invalid index")
	// not found
	ErrBlockNotFound = errors.New("block not found")
	ErrCodeNotFound  = errors.New("contract code not found")
	ErrAbiNotFound   = errors.New("abi not found")
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
	if tag := blockHash.Tag; tag != nil {
		calldata := make([]string, len(request.CallData))
		for i, data := range request.CallData {
			calldata[i] = data.Hex()
		}
		call := feeder.InvokeFunction{
			ContractAddress:    request.ContractAddress.Hex(),
			EntryPointSelector: request.EntryPointSelector.Hex(),
			Calldata:           calldata,
		}
		result, err := feederClient.CallContract(call, "", string(*tag))
		if err := returnRPCErrResponse("StarknetCall", result, err); err != nil {
			return nil, err
		}
		return (*result)["result"], nil
	}
	// notest
	return []string{"Response", "of", "starknet_call"}, nil
}

// TODO documentation
func getBlockByTag(_ context.Context, blockTag BlockTag, scope RequestedScope) (*BlockResponse, error) {
	// TODO we should the block service keep track of the latest block we have received (if we are already fully synced)
	// Then we only need to request pending blocks from the feeder gateway.

	// We basically reimplement NewBlockResponse since we have convert the type from the feeder gateway
	res, err := feederClient.GetBlock("", string(blockTag))
	if err != nil {
		// notest
		return nil, err
	}
	response := &BlockResponse{
		BlockHash:   types.HexToBlockHash(res.BlockHash),
		ParentHash:  types.HexToBlockHash(res.ParentBlockHash),
		BlockNumber: uint64(res.BlockNumber),
		Status:      types.StringToBlockStatus(res.Status),
		Sequencer:   types.HexToAddress(res.SequencerAddress),
		NewRoot:     types.HexToFelt(res.StateRoot),
	}
	if scope == ScopeTxnHash {
		txs := make([]*types.TransactionHash, len(res.Transactions))
		for i, tx := range res.Transactions {
			hash := types.HexToTransactionHash(tx.TransactionHash)
			txs[i] = &hash
		}
		response.Transactions = txs
		return response, nil
	}

	txs := make([]*Txn, len(res.Transactions))
	for i, tx := range res.Transactions {
		switch tx.Type {
		case "INVOKE":
			calldata := make([]types.Felt, len(tx.Calldata))
			for j, data := range tx.Calldata {
				calldata[j] = types.HexToFelt(data)
			}
			txs[i] = &Txn{
				FunctionCall: FunctionCall{
					ContractAddress:    types.HexToAddress(tx.ContractAddress),
					EntryPointSelector: types.HexToFelt(tx.EntryPointSelector),
					CallData:           calldata,
				},
				TxnHash: types.HexToTransactionHash(tx.TransactionHash),
			}
		// notest
		default:
			// notest
			txs[i] = &Txn{}
		}
	}

	if scope == ScopeFullTxns {
		response.Transactions = txs
		return response, nil
	}

	txnAndReceipts := make([]*TxnAndReceipt, len(res.TransactionReceipts))
	for i, feederReceipt := range res.TransactionReceipts {
		messagesSent := make([]*MsgToL1, len(feederReceipt.L2ToL1Messages))
		for j, msg := range feederReceipt.L2ToL1Messages {
			payload := make([]types.Felt, len(msg.Payload))
			for k, data := range msg.Payload {
				payload[k] = types.HexToFelt(data)
			}
			messagesSent[j] = &MsgToL1{
				ToAddress: types.HexToEthAddress(msg.ToAddress),
				Payload:   payload,
			}
		}
		events := make([]*Event, len(feederReceipt.Events))
		for j, event := range feederReceipt.Events {
			keys := make([]types.Felt, len(event.Keys))
			for k, key := range event.Keys {
				keys[k] = types.HexToFelt(key)
			}
			data := make([]types.Felt, len(event.Data))
			for k, datum := range event.Data {
				data[k] = types.HexToFelt(datum)
			}
			events[j] = &Event{
				FromAddress: types.HexToAddress(event.FromAddress),
				EventContent: EventContent{
					Keys: keys,
					Data: data,
				},
			}
		}
		payload := make([]types.Felt, len(feederReceipt.Payload))
		for j, data := range feederReceipt.Payload {
			payload[j] = types.HexToFelt(data)
		}
		txnAndReceipts[i] = &TxnAndReceipt{
			Txn: *txs[i],
			TxnReceipt: TxnReceipt{
				TxnHash:         txs[i].TxnHash,
				Status:          0,
				StatusData:      "",
				MessagesSent:    messagesSent,
				L1OriginMessage: &MsgToL2{FromAddress: types.HexToEthAddress(feederReceipt.FromAddress), Payload: payload},
				Events:          events,
			},
		}
	}
	response.Transactions = txnAndReceipts

	return response, nil
}

func getBlockByHash(ctx context.Context, blockHash types.BlockHash, scope RequestedScope) (*BlockResponse, error) {
	var dbBlock *types.Block
	var err error
	Logger.With("blockHash", blockHash, "scope", scope).Info("StarknetGetBlockByHash")

	if dbBlock, err = services.BlockService.GetBlockByHash(blockHash); err != nil {
		return nil, err
	}

	return NewBlockResponse(dbBlock, scope)
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
	block, err := getBlockByHashOrTag(ctx, blockHashOrTag, ScopeTxnHash)
	if err := returnRPCErrResponse(
		"StarknetGetBlockByHash",
		block,
		err,
	); err != nil {
		return nil, err
	}
	return block, nil
}

// StarknetGetBlockByHashOpt represent the handler for getting a block
// by its hash.
func (HandlerRPC) StarknetGetBlockByHashOpt(ctx context.Context, blockHashOrTag BlockHashOrTag, scope RequestedScope) (*BlockResponse, error) {
	block, err := getBlockByHashOrTag(ctx, blockHashOrTag, scope)
	if err := returnRPCErrResponse(
		"StarknetGetBlockByHashOpt",
		block,
		err,
	); err != nil {
		return nil, err
	}
	return block, nil
}

func getBlockByNumber(ctx context.Context, blockNumber uint64, scope RequestedScope) (*BlockResponse, error) {
	var dbBlock *types.Block
	var err error
	Logger.With("blockNumber", blockNumber, "scope", scope).Info("StarknetGetBlockNyNumber")

	if dbBlock, err = services.BlockService.GetBlockByNumber(blockNumber); err != nil {
		return nil, err
	}
  
	if dbBlock == nil {
		// notest
		return nil, ErrBlockNotFound
	}

	return NewBlockResponse(dbBlock, scope)
}

func getBlockByNumberOrTag(ctx context.Context, blockNumberOrTag BlockNumberOrTag, scope RequestedScope) (*BlockResponse, error) {
	if number := blockNumberOrTag.Number; number != nil {
		return getBlockByNumber(ctx, *number, scope)
	}
	if tag := blockNumberOrTag.Tag; tag != nil {
		return getBlockByTag(ctx, *tag, scope)
	}
	// TODO: Send bad request error
	// notest
	return nil, errors.New("bad request")
}

// StarknetGetBlockByNumber represent the handler for getting a block by
// its number.
func (HandlerRPC) StarknetGetBlockByNumber(ctx context.Context, blockNumberOrTag BlockNumberOrTag) (*BlockResponse, error) {
	// notest
	block, err := getBlockByNumberOrTag(ctx, blockNumberOrTag, ScopeTxnHash)
	if err := returnRPCErrResponse(
		"StarknetGetBlockByNumber",
		block,
		err,
	); err != nil {
		return nil, err
	}
	return block, nil
}

// StarknetGetBlockByNumberOpt represent the handler for getting a block
// by its number.
func (HandlerRPC) StarknetGetBlockByNumberOpt(ctx context.Context, blockNumberOrTag BlockNumberOrTag, scope RequestedScope) (*BlockResponse, error) {
	block, err := getBlockByNumberOrTag(ctx, blockNumberOrTag, scope)
	if err := returnRPCErrResponse(
		"StarknetGetBlockByNumberOpt",
		block,
		err,
	); err != nil {
		return nil, err
	}
	return block, nil
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
	var blockNumber uint64
	controllerName := "StarknetGetStorageAt"

	if hash := blockHash.Hash; hash != nil {
		var block *types.Block
		var err error
		block, err = services.BlockService.GetBlockByHash(*blockHash.Hash)
		if err = returnRPCErrResponse(controllerName, block, err); err != nil {
			return "", err
		}

		blockNumber = block.BlockNumber
	} else if tag := blockHash.Tag; tag != nil {
		blockResponse, err := getBlockByTag(c, *tag, ScopeTxnHash)
		if err = returnRPCErrResponse(controllerName, blockResponse, err); err != nil {
			return "", err
		}
		blockNumber = blockResponse.BlockNumber
	} else {
		// notest
		return "", returnRPCErrResponse(controllerName, nil, ErrBlockHashOrTag)
	}

	storage, err := services.StateService.GetStorage(string(contractAddress), blockNumber)
	if err = returnRPCErrResponse(controllerName, storage, err); err != nil {
		return "", err
	}

	return Felt(storage.Storage[string(key)]), nil
}

// StarknetGetTransactionByHash Get the details and status of a
// submitted transaction.
func (HandlerRPC) StarknetGetTransactionByHash(
	c context.Context, transactionHash types.TransactionHash,
) (*Txn, error) {
	tx, err := services.TransactionService.GetTransaction(transactionHash)
	if err = returnRPCErrResponse(
		"StarknetGetTransactionByHash",
		tx,
		err,
	); err != nil {
		return nil, err
	}
	return NewTxn(tx), nil
}

// StarknetGetTransactionByBlockHashAndIndex Get the details of the
// transaction given by the identified block and index in that block. If
// no transaction is found, a null value is returned.
func (HandlerRPC) StarknetGetTransactionByBlockHashAndIndex(c context.Context, blockHashOrTag BlockHashOrTag, index int) (*Txn, error) {
	controllerName := "StarknetGetTransactionByBlockHashAndIndex"
	if blockHash := blockHashOrTag.Hash; blockHash != nil {
		block, err := services.BlockService.GetBlockByHash(*blockHash)
		if err := returnRPCErrResponse(controllerName, block, err); err != nil {
			return nil, err
		}

		if index < 0 || len(block.TxHashes) <= index {
			// notest
			return nil, returnRPCErrResponse(
				controllerName,
				nil,
				fmt.Errorf("%w with value %d", ErrBlockIndex, index),
			)
		}

		txHash := block.TxHashes[index]
		txn, err := services.TransactionService.GetTransaction(txHash)
		if err := returnRPCErrResponse(controllerName, txn, err); err != nil {
			return nil, err
		}

		return NewTxn(txn), nil
	}
	// notest
	if tag := blockHashOrTag.Tag; tag != nil {
		blockResponse, err := getBlockByTag(c, *tag, ScopeFullTxns)
		if err := returnRPCErrResponse(controllerName, blockResponse, err); err != nil {
			return nil, err
		}

		txs := blockResponse.Transactions.([]*Txn)
		return txs[index], nil
	}

	return nil, returnRPCErrResponse(controllerName, nil, ErrBlockHashOrTag)
}

// StarknetGetTransactionByBlockNumberAndIndex Get the details of the
// transaction given by the identified block and index in that block. If
// no transaction is found, null is returned.
func (HandlerRPC) StarknetGetTransactionByBlockNumberAndIndex(ctx context.Context, blockNumberOrTag BlockNumberOrTag, index int) (*Txn, error) {
	var err error
	var block *types.Block
	controllerName := "StarknetGetTransactionByBlockNumberAndIndex"

	if blockNumber := blockNumberOrTag.Number; blockNumber != nil {
		block, err = services.BlockService.GetBlockByNumber(*blockNumber)
		if err = returnRPCErrResponse(controllerName, block, err); err != nil {
			return nil, err
		}

		if index < 0 || len(block.TxHashes) <= index {
			// notest
			return nil, returnRPCErrResponse(
				controllerName,
				nil,
				fmt.Errorf("%w with value %d", ErrBlockIndex, index),
			)
		}

		txHash := block.TxHashes[index]
		txn, err := services.TransactionService.GetTransaction(txHash)
		if err := returnRPCErrResponse(controllerName, txn, err); err != nil {
			return nil, err
		}

		return NewTxn(txn), nil
	}

	if tag := blockNumberOrTag.Tag; tag != nil {
		blockResponse, err := getBlockByTag(ctx, *tag, ScopeFullTxns)
		if err := returnRPCErrResponse(controllerName, blockResponse, err); err != nil {
			return nil, err
		}

		txs := blockResponse.Transactions.([]*Txn)
		return txs[index], nil
	}

	// notest
	return nil, returnRPCErrResponse(controllerName, nil, ErrBlockHashOrTag)
}

// StarknetGetTransactionReceipt Get the transaction receipt by the
// transaction hash.
func (HandlerRPC) StarknetGetTransactionReceipt(
	c context.Context, transactionHash types.TransactionHash,
) (TxnReceipt, error) {
	return TxnReceipt{}, nil
}

// StarknetGetCode Get the code of a specific contract
func (HandlerRPC) StarknetGetCode(
	c context.Context, contractAddress types.Address,
) (*CodeResult, error) {
	controllerName := "StarknetGetCode"

	abi, err := services.AbiService.GetAbi(contractAddress.Hex())
	if err := returnRPCErrResponse(controllerName, abi, err); err != nil {
		return nil, err
	}
	if abi == nil {
		// Try the feeder gateway for pending block
		code, err := feederClient.GetCode(contractAddress.Felt().String(), "", string(BlocktagPending))
		if err := returnRPCErrResponse(controllerName, code, err); err != nil {
			return nil, err
		}
		// Convert feeder type to RPC CodeResult type
		bytecode := make([]types.Felt, len(code.Bytecode))
		for i, code := range code.Bytecode {
			bytecode[i] = types.HexToFelt(code)
		}
		marshal, err := json.Marshal(code.Abi)
		if err := returnRPCErrResponse(controllerName, marshal, err); err != nil {
			return nil, err
		}
		var abiResponse dbAbi.Abi
		err = json.Unmarshal(marshal, &abiResponse)
		if err := returnRPCErrResponse(controllerName, abiResponse, err); err != nil {
			return nil, err
		}
		for i, str := range code.Abi.Structs {
			abiResponse.Structs[i].Fields = make([]*dbAbi.Struct_Field, len(str.Members))
			for j, field := range str.Members {
				abiResponse.Structs[i].Fields[j] = &dbAbi.Struct_Field{
					Name:   field.Name,
					Type:   field.Type,
					Offset: uint32(field.Offset),
				}
			}
		}
		marshal, err = json.Marshal(abiResponse)
		if err := returnRPCErrResponse(controllerName, marshal, err); err != nil {
			return nil, err
		}

		return &CodeResult{Abi: string(marshal), Bytecode: bytecode}, nil
	}

	code, err := services.StateService.GetCode(contractAddress.Bytes())
	if err := returnRPCErrResponse(controllerName, code, err); err != nil {
		return nil, err
	}

	marshalledAbi, err := json.Marshal(abi)
	if err := returnRPCErrResponse(controllerName, marshalledAbi, err); err != nil {
		return nil, err
	}

	bytecode := make([]types.Felt, len(code.Code))
	for i, b := range code.Code {
		bytecode[i] = types.BytesToFelt(b)
	}

	return &CodeResult{Abi: string(marshalledAbi), Bytecode: bytecode}, nil
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
) ([]*Txn, error) {
	block, err := getBlockByTag(c, BlocktagPending, ScopeFullTxns)
	if err := returnRPCErrResponse(
		"StarknetPendingTransactions",
		block,
		err,
	); err != nil {
		return nil, err
	}
	return block.Transactions.([]*Txn), nil
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

//util function
func returnRPCErrResponse(controllerName string, obj interface{}, err error) error {
	if err != nil {
		Logger.Errorf("%s: %s", controllerName, err)
		if errors.As(err, &ErrCodeNotFound) {
			return ErrCodeNotFound
		}
		if errors.As(err, &ErrBlockNotFound) {
			return ErrBlockNotFound
		}
		if errors.As(err, &ErrAbiNotFound) {
			return ErrAbiNotFound
		}
		if errors.As(err, &ErrBlockHashOrTag) {
			return ErrInvalidParams()
		}
		if errors.As(err, &ErrBlockIndex) {
			return ErrInvalidParams()
		}
		return ErrInternal()
	}

	if obj == nil {
		return ErrResourceNotFound()
	}

	return nil
}
