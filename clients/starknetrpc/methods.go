package starknetrpc

import (
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
)

type params interface {
	args() []any
}

type NoParams struct{}

func (NoParams) args() []any { return nil }

type BlockParams struct {
	BlockNumber uint64
}

func (p BlockParams) args() []any {
	return []any{rpcv10.BlockIDFromNumber(p.BlockNumber)}
}

// Proof facts are part of the INVOKE v3 transaction hash; the remote omits them unless asked.
type BlockWithReceiptsParams struct {
	BlockNumber uint64
}

func (p BlockWithReceiptsParams) args() []any {
	return []any{rpcv10.BlockIDFromNumber(p.BlockNumber), []string{"INCLUDE_PROOF_FACTS"}}
}

type ClassParams struct {
	BlockNumber uint64
	ClassHash   *felt.Felt
}

func (p ClassParams) args() []any {
	return []any{rpcv10.BlockIDFromNumber(p.BlockNumber), p.ClassHash}
}

type ClassHashParams struct {
	ClassHash *felt.Felt
}

func (p ClassHashParams) args() []any { return []any{p.ClassHash} }

type method[P params, R any] struct {
	name string
}

var (
	ChainID            = method[NoParams, felt.Felt]{name: "starknet_chainId"}
	SpecVersion        = method[NoParams, string]{name: "starknet_specVersion"}
	BlockHashAndNumber = method[NoParams, rpcv10.BlockHashAndNumber]{
		name: "starknet_blockHashAndNumber",
	}
	BlockWithReceipts = method[BlockWithReceiptsParams, rpcv10.BlockWithReceipts]{
		name: "starknet_getBlockWithReceipts",
	}
	StateUpdate  = method[BlockParams, rpcv10.StateUpdate]{name: "starknet_getStateUpdate"}
	Class        = method[ClassParams, rpcv10.Class]{name: "starknet_getClass"}
	CompiledCasm = method[ClassHashParams, rpcv10.CompiledCasmResponse]{
		name: "starknet_getCompiledCasm",
	}
)
