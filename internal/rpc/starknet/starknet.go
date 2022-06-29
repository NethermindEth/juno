package starknet

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/pkg/types"
)

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

func (s *StarkNetRpc) GetBlockByHash(ctx context.Context, params *GetBlockByHashParams) (any, error) {
	// TODO: implement
	return params, nil
}

type GetBlockByNumberPrams struct {
    BlockNumberOrTag BlockNumberOrTag `json:"block_number"`
    Scope            RequestedScope   `json:"scope"`
}

func (s *StarkNetRpc) GetBlockByNumber(ctx context.Context, params *GetBlockByNumberPrams) (any, error) {
    // TODO: implement
    return params, nil
}

type GetStorageAtParams struct {
    ContractAddress types.Address `json:"contract_address"`
    Key             types.Felt    `json:"key"`
    BlockHashOrTag  BlockHashOrTag `json:"block_hash"`
}

func (s *StarkNetRpc) GetStorageAt(ctx context.Context, params *GetStorageAtParams) (any, error) {
    // TODO: implement
    return params, nil
}

type GetTransactionByHashParams struct {
    TransactionHash types.TransactionHash `json:"transaction_hash"`
}

func (s *StarkNetRpc) GetTransactionByHash(ctx context.Context, params *GetTransactionByHashParams) (any, error) {
    // TODO: implement
    return params, nil
}

type GetTransactionByBlockHashAndIndexParams struct {
    BlockHashOrTag BlockHashOrTag `json:"block_hash"`
    Index int `json:"index"`
}

func (s *StarkNetRpc) GetTransactionByBlockHashAndIndex(ctx context.Context, params *GetTransactionByBlockHashAndIndexParams) (any, error) {
    // TODO: implement
    return params, nil
}

type GetTransactionByBlockNumberAndIndexParams struct {
    BlockNumberOrTag BlockNumberOrTag `json:"block_number"`
    Index int `json:"index"`
}

func (s *StarkNetRpc) GetTransactionByBlockNumberAndIndex(ctx context.Context, params *GetTransactionByBlockNumberAndIndexParams) (any, error) {
    // TODO: impleement
    return params, nil
}

type GetCodeParam struct {
    ContractAddress types.Address `json:"contract_address"`
}

func (s *StarkNetRpc) GetCode(ctx context.Context, params *GetCodeParam) (any, error) {
    // TODO: implement
    return params, nil
}







