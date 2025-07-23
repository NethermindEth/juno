package rpcv9

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
)

/****************************************************
		Nonce Handler
*****************************************************/

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/25533f8b7999219120a4a654709d47dc9376d640/api/starknet_api_openrpc.json#L851
func (h *Handler) Nonce(id *BlockID, address *felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	nonce, err := stateReader.ContractNonce(address)
	if err != nil {
		return nil, rpccore.ErrContractNotFound
	}

	return &nonce, nil
}
