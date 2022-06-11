package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/NethermindEth/juno/pkg/feeder"

	"github.com/NethermindEth/juno/internal/log"
)

// Global feederClient that we use to request pending blocks
var feederClient = feeder.NewClient("https://alpha-mainnet.starknet.io", "/feeder_gateway", nil)

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

// TODO documentation
func getBlockByTag(_ context.Context, blockTag BlockTag, scope RequestedScope) (*BlockResponse, error) {
	// TODO we should the block service keep track of the latest block we have received (if we are already fully synced)
	// Then we only need to request pending blocks from the feeder gateway.

	// We basically reimplement NewBlockResponse since we have convert the type from the feeder gateway
	res, err := feederClient.GetBlock("", string(blockTag))
	if err != nil {
		return nil, err
	}
	response := &BlockResponse{
		BlockHash: types.HexToBlockHash(res.BlockHash),
		ParentHash: types.HexToBlockHash(res.ParentBlockHash),
		BlockNumber: uint64(res.BlockNumber),
		Status: types.StringToBlockStatus(res.Status),
		Sequencer: types.HexToAddress(res.SequencerAddress),
		NewRoot: types.HexToFelt(res.StateRoot),
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
		default:
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
				Payload: payload,	
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
		payload := make([]types.Felt, len(feederReceipt.L1ToL2ConsumedMessage.Payload))
		for j, data := range feederReceipt.L1ToL2ConsumedMessage.Payload {
			payload[j] = types.HexToFelt(data)
		}
		txnAndReceipts[i] = &TxnAndReceipt{
			Txn: *txs[i],
			TxnReceipt: TxnReceipt{
				TxnHash: txs[i].TxnHash,
				MessagesSent: messagesSent,
				L1OriginMessage: &MsgToL2{
					FromAddress: types.HexToEthAddress(feederReceipt.L1ToL2ConsumedMessage.FromAddress),
					Payload: payload,
				},
				Events: events,
			},
		}
	}
	response.Transactions = txnAndReceipts

	return response, nil
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
	var blockNumber uint64
	if hash := blockHash.Hash; hash != nil {
		block := services.BlockService.GetBlockByHash(*blockHash.Hash)
		blockNumber = block.BlockNumber
		if block == nil {
			// notest
			return "", fmt.Errorf("block not found")
		}
	} else if tag := blockHash.Tag; tag != nil {
		// notest
		blockResponse, err := getBlockByTag(c, *tag, ScopeTxnHash)
		blockNumber = blockResponse.BlockNumber
		if err != nil {
			// notest
			return "", fmt.Errorf("block not found")
		}
	} else {
		return "", fmt.Errorf("invalid block hash or tag")
	}

	storage := services.StateService.GetStorage(string(contractAddress), blockNumber)
	if storage == nil {
		// notest
		return "", fmt.Errorf("storage not found")
	}

	return Felt(storage.Storage[string(key)]), nil
}

// StarknetGetTransactionByHash Get the details and status of a
// submitted transaction.
func (HandlerRPC) StarknetGetTransactionByHash(
	c context.Context, transactionHash types.TransactionHash,
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
	c context.Context, transactionHash types.TransactionHash,
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
	code := services.StateService.GetCode(contractAddress.Bytes())
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
	block, err := getBlockByTag(c, "pending", ScopeFullTxns)
	if err != nil {
		return nil, err
	}
	return block.Transactions.([]Txn), nil
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
