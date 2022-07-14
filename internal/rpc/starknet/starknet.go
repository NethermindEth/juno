package starknet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	dbAbi "github.com/NethermindEth/juno/internal/db/abi"

	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/types"
)

// Global feederClient that we use to request pending blocks
var feederClient *feeder.Client

type StarkNetRpc struct{}

type EchoParams struct {
	Message string
}

func (s *StarkNetRpc) Echo(ctx context.Context, params *EchoParams) (any, error) {
	return params.Message, nil
}

func (s *StarkNetRpc) EchoError(ctx context.Context, params *EchoParams) (any, error) {
	return nil, fmt.Errorf(params.Message)
}

type CallParams struct {
	ContractAddress    types.Address  `json:"contract_address"`
	EntryPointSelector types.Felt     `json:"entry_point_selector"`
	CallData           []types.Felt   `json:"calldata"`
	BlockHashOrtag     BlockHashOrTag `json:"block_hash"`
}

func (s *StarkNetRpc) Call(ctx context.Context, params *CallParams) (any, error) {
	// TODO: implement
	return params, nil
}

type GetBlockByHashParams struct {
	BlockHashOrTag BlockHashOrTag `json:"block_hash"`
	Scope          RequestedScope `json:"scope"`
}

func getBlockByHash(ctx context.Context, blockHash types.BlockHash, scope RequestedScope) (*BlockResponse, error) {
	log.Default.With("blockHash", blockHash, "scope", scope).Info("StarknetGetBlockByHash")
	dbBlock := services.BlockService.GetBlockByHash(blockHash)
	if dbBlock == nil {
		// notest
		// TODO: Send custom error for not found. Maybe sent InvalidBlockHash?
		return nil, NewInvalidBlockHash()
	}
	response := NewBlockResponse(dbBlock, scope)
	return response, nil
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

func (s *StarkNetRpc) GetBlockByHash(ctx context.Context, params *GetBlockByHashParams) (any, error) {
	// Set default scope if it comes as empty
	if params.Scope == "" {
		params.Scope = ScopeTxnHash
	}
	if params.BlockHashOrTag.Hash != nil {
		return getBlockByHash(ctx, *params.BlockHashOrTag.Hash, params.Scope)
	}
	if params.BlockHashOrTag.Tag != nil {
		return getBlockByTag(ctx, *params.BlockHashOrTag.Tag, params.Scope)
	}
	return params, nil
}

type GetBlockByNumberPrams struct {
	BlockNumberOrTag BlockNumberOrTag `json:"block_number"`
	Scope            RequestedScope   `json:"scope"`
}

func (s *StarkNetRpc) GetBlockByNumber(ctx context.Context, params *GetBlockByNumberPrams) (any, error) {
	if params.Scope == "" {
		params.Scope = ScopeTxnHash
	}
	return getBlockByNumberOrTag(ctx, params.BlockNumberOrTag, params.Scope)
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

type GetStorageAtParams struct {
	ContractAddress types.Address  `json:"contract_address"`
	Key             types.Felt     `json:"key"`
	BlockHashOrTag  BlockHashOrTag `json:"block_hash"`
}

func (s *StarkNetRpc) GetStorageAt(ctx context.Context, params *GetStorageAtParams) (any, error) {
	var blockNumber uint64
	if hash := params.BlockHashOrTag.Hash; hash != nil {
		block := services.BlockService.GetBlockByHash(*hash)
		if block == nil {
			// notest
			return "", fmt.Errorf("block not found")
		}
		blockNumber = block.BlockNumber
	} else if tag := params.BlockHashOrTag.Tag; tag != nil {
		blockResponse, err := getBlockByTag(ctx, *tag, ScopeTxnHash)
		if err != nil {
			// notest
			return "", fmt.Errorf("block not found")
		}
		blockNumber = blockResponse.BlockNumber
	} else {
		// notest
		return "", fmt.Errorf("invalid block hash or tag")
	}

	storage := services.StateService.GetStorage(params.ContractAddress.Hex(), blockNumber)
	if storage == nil {
		// notest
		return "", fmt.Errorf("storage not found")
	}

	return storage.Storage[params.Key.HexWithout0x()], nil
}

type GetTransactionByHashParams struct {
	TransactionHash types.TransactionHash `json:"transaction_hash"`
}

func (s *StarkNetRpc) GetTransactionByHash(ctx context.Context, params *GetTransactionByHashParams) (any, error) {
	tx := services.TransactionService.GetTransaction(params.TransactionHash)
	if tx == nil {
		// notest
		// TODO: return not found error
		return &Txn{}, nil
	}
	return NewTxn(tx), nil
}

type GetTransactionByBlockHashAndIndexParams struct {
	BlockHashOrTag BlockHashOrTag `json:"block_hash"`
	Index          int            `json:"index"`
}

func (s *StarkNetRpc) GetTransactionByBlockHashAndIndex(ctx context.Context, params *GetTransactionByBlockHashAndIndexParams) (any, error) {
	if blockHash := params.BlockHashOrTag.Hash; blockHash != nil {
		block := services.BlockService.GetBlockByHash(*blockHash)
		// TODO check if block is nil?
		if params.Index < 0 || len(block.TxHashes) <= params.Index {
			// notest
			return nil, fmt.Errorf("invalid index %d", params.Index)
		}
		txHash := block.TxHashes[params.Index]
		txn := services.TransactionService.GetTransaction(txHash)
		return NewTxn(txn), nil
	}
	// notest
	if tag := params.BlockHashOrTag.Tag; tag != nil {
		blockResponse, err := getBlockByTag(ctx, *tag, ScopeFullTxns)
		if err != nil {
			// notest
			return nil, fmt.Errorf("block not found")
		}
		txs := blockResponse.Transactions.([]*Txn)
		return txs[params.Index], nil
	}
	// TODO: return invalid param error
	return nil, errors.New("invalid blockHashOrtTag param")
}

type GetTransactionByBlockNumberAndIndexParams struct {
	BlockNumberOrTag BlockNumberOrTag `json:"block_number"`
	Index            int              `json:"index"`
}

func (s *StarkNetRpc) GetTransactionByBlockNumberAndIndex(ctx context.Context, params *GetTransactionByBlockNumberAndIndexParams) (any, error) {
	if blockNumber := params.BlockNumberOrTag.Number; blockNumber != nil {
		block := services.BlockService.GetBlockByNumber(*blockNumber)
		if params.Index < 0 || len(block.TxHashes) <= params.Index {
			// notest
			return nil, fmt.Errorf("invalid index %d", params.Index)
		}
		txHash := block.TxHashes[params.Index]
		txn := services.TransactionService.GetTransaction(txHash)
		return NewTxn(txn), nil
	}
	if tag := params.BlockNumberOrTag.Tag; tag != nil {
		blockResponse, err := getBlockByTag(ctx, *tag, ScopeFullTxns)
		if err != nil {
			// notest
			return nil, fmt.Errorf("block not found")
		}
		txs := blockResponse.Transactions.([]*Txn)
		return txs[params.Index], nil
	}
	// TODO: return invalid param error
	// notest
	return nil, errors.New("invalid blockHashOrtTag param")
}

type GetCodeParam struct {
	ContractAddress types.Address `json:"contract_address"`
}

func (s *StarkNetRpc) GetCode(ctx context.Context, params *GetCodeParam) (any, error) {
	abi := services.AbiService.GetAbi(params.ContractAddress.Hex())
	if abi == nil {
		// Try the feeder gateway for pending block
		code, err := feederClient.GetCode(params.ContractAddress.Felt().String(), "", string(BlocktagPending))
		if err != nil {
			// notest
			return nil, fmt.Errorf("abi not found %v", err)
		}
		// Convert feeder type to RPC CodeResult type
		bytecode := make([]types.Felt, len(code.Bytecode))
		for i, code := range code.Bytecode {
			bytecode[i] = types.HexToFelt(code)
		}
		marshal, err := json.Marshal(code.Abi)
		if err != nil {
			// notest
			return nil, fmt.Errorf("abi not found %v", err)
		}
		var abiResponse dbAbi.Abi
		err = json.Unmarshal(marshal, &abiResponse)
		if err != nil {
			// notest
			return nil, fmt.Errorf("abi not found %v", err)
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
		marshal, err = json.Marshal(&abiResponse)
		if err != nil {
			// notest
			return nil, fmt.Errorf("unexpected marshal error %v", err)
		}
		return &CodeResult{Abi: string(marshal), Bytecode: bytecode}, nil
	}
	code := services.StateService.GetCode(params.ContractAddress.Bytes())
	if code == nil {
		// notest
		return nil, fmt.Errorf("code not found")
	}
	marshalledAbi, err := json.Marshal(abi)
	if err != nil {
		// notest
		return nil, err
	}
	bytecode := make([]types.Felt, len(code.Code))
	for i, b := range code.Code {
		bytecode[i] = types.BytesToFelt(b)
	}
	return &CodeResult{Abi: string(marshalledAbi), Bytecode: bytecode}, nil
}
