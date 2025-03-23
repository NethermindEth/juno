package rpcv7

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

/****************************************************
		Contract Handlers
*****************************************************/

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L110
func (h *Handler) StorageAt(address, key felt.Felt, id BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// This checks if the contract exists because if a key doesn't exist in contract storage,
	// the returned value is always zero and error is nil.
	_, err := stateReader.ContractClassHash(address)
	if err != nil {
		if errors.Is(err, state.ErrContractNotDeployed) {
			return nil, rpccore.ErrContractNotFound
		}
		h.log.Errorw("Failed to get contract nonce", "err", err)
		return nil, rpccore.ErrInternal
	}

	value, err := stateReader.ContractStorage(address, key)
	if err != nil {
		return nil, rpccore.ErrInternal
	}

	return &value, nil
}
